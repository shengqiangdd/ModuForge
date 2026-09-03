package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

// BuildAndroidAppSkill builds an Android APP (APK) from a generated project.
type BuildAndroidAppSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter
}

// NewBuildAndroidAppSkillWithDB creates a new BuildAndroidAppSkill with database support.
func NewBuildAndroidAppSkillWithDB(projectPath string, db *sql.DB) *BuildAndroidAppSkill {
	return &BuildAndroidAppSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter.
func (s *BuildAndroidAppSkill) WithStorage(st storage.StorageAdapter) *BuildAndroidAppSkill {
	s.storage = st
	return s
}

func (s *BuildAndroidAppSkill) Name() string {
	return "build_android_app"
}

func (s *BuildAndroidAppSkill) Description() string {
	return `Build Android APP (APK) from a generated project.
Input: {"project_id": "...", "app_dir": "app"}.
Compiles Kotlin with Gradle, signs with debug keystore, copies APK to module root/app/.
Returns build log and APK path.`
}

func (s *BuildAndroidAppSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	projectID, _ := input["project_id"].(string)
	appDir, _ := input["app_dir"].(string)
	if appDir == "" {
		appDir = "app"
	}

	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}

	projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
	gradleProjectDir := filepath.Join(projectPath, appDir)

	// Check if app project exists
	// android_app tool creates project structure at appDir/ directly (e.g., app/build.gradle.kts)
	buildGradleKts := filepath.Join(gradleProjectDir, "build.gradle.kts")
	if _, err := os.Stat(buildGradleKts); os.IsNotExist(err) {
		return "", fmt.Errorf("app project not found at %s — run android_app skill first", buildGradleKts)
	}

	var log strings.Builder
	log.WriteString("🔨 Build Android APP\n")
	log.WriteString(fmt.Sprintf("📂 Project: %s\n", gradleProjectDir))

	// Ensure Gradle wrapper exists
	s.ensureGradleWrapper(gradleProjectDir)

	// Try using full Gradle installation first, fall back to gradlew
	gradleBin := "/opt/gradle/gradle-8.7/bin/gradle"
	if _, err := os.Stat(gradleBin); os.IsNotExist(err) {
		gradleBin = filepath.Join(gradleProjectDir, "gradlew")
	}

	// Run Gradle assembleDebug
	log.WriteString("\n── Compiling APK ──\n")
	buildCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(buildCtx, gradleBin, "assembleDebug", "--no-daemon", "--stacktrace")
	cmd.Dir = gradleProjectDir
	cmd.Env = append(os.Environ(),
		"ANDROID_HOME=/opt/android-sdk",
		"ANDROID_SDK_ROOT=/opt/android-sdk",
		"JAVA_HOME=/usr/lib/jvm/java-17-openjdk",
		"GRADLE_OPTS=-Xmx1536m",
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		log.WriteString(fmt.Sprintf("❌ Gradle build failed:\n%s\n", outputStr))
		return log.String(), fmt.Errorf("gradle build failed: %w", err)
	}
	log.WriteString("✅ Gradle build succeeded\n")

	// Find the generated APK
	apkPath := filepath.Join(gradleProjectDir, "app", "build", "outputs", "apk", "debug", "app-debug.apk")
	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		// Try alternate path
		apkPath = filepath.Join(gradleProjectDir, "app", "build", "outputs", "apk", "debug", "app-debug-unsigned.apk")
		if _, err2 := os.Stat(apkPath); os.IsNotExist(err2) {
			log.WriteString("❌ APK not found in expected output directory\n")
			return log.String(), fmt.Errorf("APK output not found")
		}
	}

	// Copy APK to module root/app/
	apkDestDir := filepath.Join(projectPath, "app")
	os.MkdirAll(apkDestDir, 0755)
	apkDest := filepath.Join(apkDestDir, "app.apk")

	apkData, err := os.ReadFile(apkPath)
	if err != nil {
		log.WriteString(fmt.Sprintf("❌ Failed to read APK: %v\n", err))
		return log.String(), err
	}
	if err := os.WriteFile(apkDest, apkData, 0644); err != nil {
		log.WriteString(fmt.Sprintf("❌ Failed to copy APK: %v\n", err))
		return log.String(), err
	}

	apkSizeMB := float64(len(apkData)) / 1024 / 1024
	log.WriteString(fmt.Sprintf("✅ APK copied to %s (%.1f MB)\n", apkDest, apkSizeMB))

	// Upload APK to S3 if storage is configured
	if s.storage != nil && s.db != nil {
		relPath := filepath.Join(appDir, "app", "build", "outputs", "apk", "debug", "app-debug.apk")
		if apkData, readErr := os.ReadFile(apkPath); readErr == nil {
			writeFileContent(ctx, s.storage, s.db, projectID, "app/app.apk", string(apkData))
			_ = relPath
		}
	}

	log.WriteString("\n✅ Android APP build complete!\n")
	log.WriteString(fmt.Sprintf("APK: %s\n", apkDest))

	return log.String(), nil
}

func (s *BuildAndroidAppSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true, // Always expose to all model tiers
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}

// ensureGradleWrapper creates gradlew and gradlew.bat if they don't exist.
func (s *BuildAndroidAppSkill) ensureGradleWrapper(projectDir string) {
	gradlew := filepath.Join(projectDir, "gradlew")
	if _, err := os.Stat(gradlew); err == nil {
		return
	}

	// Create gradlew script
	gradlewContent := "#!/bin/sh\n" +
		"# Gradle wrapper script\n" +
		"APP_BASE_NAME=$(basename \"$0\")\n" +
		"APP_HOME=$(cd \"$(dirname \"$0\")\" && pwd)\n" +
		"CLASSPATH=$APP_HOME/gradle/wrapper/gradle-wrapper.jar\n" +
		"JAVACMD=${JAVA_HOME:+$JAVA_HOME/bin/}java\n" +
		"exec \"$JAVACMD\" -classpath \"$CLASSPATH\" org.gradle.wrapper.GradleWrapperMain \"$@\"\n"
	os.WriteFile(gradlew, []byte(gradlewContent), 0755)

	// Create gradle wrapper properties if missing
	wrapperDir := filepath.Join(projectDir, "gradle", "wrapper")
	os.MkdirAll(wrapperDir, 0755)
	propsPath := filepath.Join(wrapperDir, "gradle-wrapper.properties")
	if _, err := os.Stat(propsPath); os.IsNotExist(err) {
		os.WriteFile(propsPath, []byte(`distributionBase=GRADLE_USER_HOME
distributionPath=wrapper/dists
distributionUrl=https\://services.gradle.org/distributions/gradle-8.5-bin.zip
networkTimeout=10000
validateDistributionUrl=true
zipStoreBase=GRADLE_USER_HOME
zipStorePath=wrapper/dists
`), 0644)
	}

	// Download gradle-wrapper.jar if missing
	jarPath := filepath.Join(wrapperDir, "gradle-wrapper.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		dlCmd := exec.Command("curl", "-sL", "-o", jarPath,
			"https://raw.githubusercontent.com/gradle/gradle/v8.5.0/gradle/wrapper/gradle-wrapper.jar")
		dlCmd.Run()
	}
}
