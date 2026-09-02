package builder

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AndroidProject represents a detected Android project.
type AndroidProject struct {
	Dir            string
	HasGradlew     bool
	HasBuildGradle bool
	HasManifest    bool
	Language       string // "kotlin" or "java"
}

// DetectAndroidProjects scans projectDir for Android projects (build.gradle or build.gradle.kts).
func (b *Builder) DetectAndroidProjects(projectDir string) []AndroidProject {
	var projects []AndroidProject

	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if name == "build.gradle" || name == "build.gradle.kts" {
			dir := filepath.Dir(path)
			ap := AndroidProject{
				Dir:            dir,
				HasGradlew:     fileExists(filepath.Join(dir, "gradlew")),
				HasBuildGradle: true,
				HasManifest:    fileExists(filepath.Join(dir, "src", "main", "AndroidManifest.xml")),
			}
			// Detect language
			if dirExists(filepath.Join(dir, "src", "main", "java")) {
				ap.Language = "java"
			} else if dirExists(filepath.Join(dir, "src", "main", "kotlin")) || dirExists(filepath.Join(dir, "src", "main", "java")) {
				ap.Language = "kotlin"
			}
			projects = append(projects, ap)
		}
		return nil
	})

	return projects
}

// BuildAndroidAPK builds an APK using Gradle.
func (b *Builder) BuildAndroidAPK(ctx context.Context, projectDir, taskID, arch string, logFn func(string)) (*BuildResult, error) {
	start := time.Now()
	result := &BuildResult{Arch: arch}

	projects := b.DetectAndroidProjects(projectDir)
	if len(projects) == 0 {
		return nil, fmt.Errorf("no Android project found (missing build.gradle or build.gradle.kts)")
	}

	ap := projects[0]
	logFn(fmt.Sprintf("  📱 Detected Android project: %s (language: %s)\n", ap.Dir, ap.Language))

	// Ensure gradlew exists
	if !ap.HasGradlew {
		logFn("  📝 Generating Gradle wrapper...\n")
		if err := generateGradleWrapper(ctx, ap.Dir, logFn); err != nil {
			return nil, fmt.Errorf("failed to generate Gradle wrapper: %w", err)
		}
	}

	// Determine Gradle task based on build type
	gradleTask := "assembleDebug"
	logFn(fmt.Sprintf("  🔨 Running: gradlew %s\n", gradleTask))

	// Build environment
	cmd := exec.CommandContext(ctx, "./gradlew", gradleTask, "--no-daemon", "--stacktrace")
	cmd.Dir = ap.Dir
	cmd.Env = append(os.Environ(),
		"ANDROID_HOME=/opt/android-sdk",
		"ANDROID_SDK_ROOT=/opt/android-sdk",
		"JAVA_HOME=/usr/lib/jvm/java-17-openjdk",
		fmt.Sprintf("GRADLE_USER_HOME=%s/.gradle", ap.Dir),
	)

	// Capture output for logging
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		// Log the last 50 lines of output for debugging
		lines := strings.Split(outputStr, "\n")
		start := 0
		if len(lines) > 50 {
			start = len(lines) - 50
		}
		logFn(fmt.Sprintf("  ❌ Gradle build failed:\n%s\n", strings.Join(lines[start:], "\n")))
		return nil, fmt.Errorf("gradle build failed: %w\nOutput:\n%s", err, outputStr)
	}

	logFn("  ✅ Gradle build succeeded\n")

	// Find APK output
	apkPath := findBuiltAPK(ap.Dir)
	if apkPath == "" {
		return nil, fmt.Errorf("APK file not found after build")
	}

	// Copy APK to artifact directory
	artifactDir := filepath.Join(b.cfg.StoragePath, "artifacts", taskID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("create artifact dir: %w", err)
	}

	apkName := filepath.Base(apkPath)
	artifactPath := filepath.Join(artifactDir, apkName)
	if err := copyFile(apkPath, artifactPath); err != nil {
		return nil, fmt.Errorf("copy APK to artifacts: %w", err)
	}

	result.ArtifactPath = artifactPath
	result.Duration = time.Since(start)

	logFn(fmt.Sprintf("  📦 APK artifact: %s\n", artifactPath))
	return result, nil
}

// generateGradleWrapper generates a Gradle wrapper in the project directory.
func generateGradleWrapper(ctx context.Context, projectDir string, logFn func(string)) error {
	cmd := exec.CommandContext(ctx, "gradle", "wrapper", "--gradle-version", "8.7")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(),
		"JAVA_HOME=/usr/lib/jvm/java-17-openjdk",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logFn(fmt.Sprintf("  ⚠️  gradle wrapper generation output: %s\n", string(output)))
		return err
	}
	// Make gradlew executable
	os.Chmod(filepath.Join(projectDir, "gradlew"), 0755)
	return nil
}

// findBuiltAPK searches for the built APK in standard Gradle output locations.
func findBuiltAPK(projectDir string) string {
	// Standard Gradle APK output paths
	candidates := []string{
		filepath.Join(projectDir, "app", "build", "outputs", "apk", "debug", "app-debug.apk"),
		filepath.Join(projectDir, "app", "build", "outputs", "apk", "release", "app-release.apk"),
		filepath.Join(projectDir, "build", "outputs", "apk", "debug", "*.apk"),
		filepath.Join(projectDir, "build", "outputs", "apk", "release", "*.apk"),
	}

	for _, pattern := range candidates {
		if strings.Contains(pattern, "*") {
			matches, _ := filepath.Glob(pattern)
			if len(matches) > 0 {
				return matches[0]
			}
		} else if fileExists(pattern) {
			return pattern
		}
	}

	// Broader search
	var found string
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".apk") {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// GenerateAndroidProject generates a minimal Android project from a template.
func GenerateAndroidProject(projectDir, name, language string, logFn func(string)) error {
	logFn(fmt.Sprintf("  📝 Generating Android project: %s (language: %s)\n", name, language))

	// Create directory structure
	dirs := []string{
		filepath.Join(projectDir, "app", "src", "main", "java", "com", "moduforge", strings.ToLower(name)),
		filepath.Join(projectDir, "app", "src", "main", "res", "layout"),
		filepath.Join(projectDir, "app", "src", "main", "res", "values"),
		filepath.Join(projectDir, "app", "src", "main", "res", "drawable"),
		filepath.Join(projectDir, "gradle", "wrapper"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	// Generate root build.gradle.kts
	if err := writeTemplate(filepath.Join(projectDir, "build.gradle.kts"), rootBuildGradleKTS(name)); err != nil {
		return err
	}

	// Generate settings.gradle.kts
	if err := writeTemplate(filepath.Join(projectDir, "settings.gradle.kts"), settingsGradleKTS(name)); err != nil {
		return err
	}

	// Generate gradle.properties
	if err := writeTemplate(filepath.Join(projectDir, "gradle.properties"), gradleProperties()); err != nil {
		return err
	}

	// Generate app/build.gradle.kts
	if err := writeTemplate(filepath.Join(projectDir, "app", "build.gradle.kts"), appBuildGradleKTS(name)); err != nil {
		return err
	}

	// Generate AndroidManifest.xml
	if err := writeTemplate(filepath.Join(projectDir, "app", "src", "main", "AndroidManifest.xml"), androidManifest(name)); err != nil {
		return err
	}

	// Generate main activity based on language
	if language == "kotlin" {
		// Create kotlin source dir
		kotlinDir := filepath.Join(projectDir, "app", "src", "main", "java", "com", "moduforge", strings.ToLower(name))
		os.MkdirAll(kotlinDir, 0755)
		if err := writeTemplate(filepath.Join(kotlinDir, "MainActivity.kt"), mainActivityKotlin(name)); err != nil {
			return err
		}
	} else {
		javaDir := filepath.Join(projectDir, "app", "src", "main", "java", "com", "moduforge", strings.ToLower(name))
		os.MkdirAll(javaDir, 0755)
		if err := writeTemplate(filepath.Join(javaDir, "MainActivity.java"), mainActivityJava(name)); err != nil {
			return err
		}
	}

	// Generate layout
	if err := writeTemplate(filepath.Join(projectDir, "app", "src", "main", "res", "layout", "activity_main.xml"), mainLayout()); err != nil {
		return err
	}

	// Generate strings.xml
	if err := writeTemplate(filepath.Join(projectDir, "app", "src", "main", "res", "values", "strings.xml"), stringsXML(name)); err != nil {
		return err
	}

	// Generate themes.xml
	if err := writeTemplate(filepath.Join(projectDir, "app", "src", "main", "res", "values", "themes.xml"), themesXML()); err != nil {
		return err
	}

	logFn("  ✅ Android project generated\n")
	return nil
}

func writeTemplate(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// --- Template functions ---

func rootBuildGradleKTS(name string) string {
	return `// Top-level build file
plugins {
    id("com.android.application") version "8.2.0" apply false
    id("org.jetbrains.kotlin.android") version "1.9.20" apply false
}
`
}

func settingsGradleKTS(name string) string {
	return fmt.Sprintf(`pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}
rootProject.name = "%s"
include(":app")
`, name)
}

func gradleProperties() string {
	return `org.gradle.jvmargs=-Xmx2048m -Dfile.encoding=UTF-8
android.useAndroidX=true
kotlin.code.style=official
android.nonTransitiveRClass=true
`
}

func appBuildGradleKTS(name string) string {
	return fmt.Sprintf(`plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.moduforge.%s"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.moduforge.%s"
        minSdk = 24
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("androidx.appcompat:appcompat:1.6.1")
    implementation("com.google.android.material:material:1.11.0")
    implementation("androidx.constraintlayout:constraintlayout:2.1.4")
}
`, strings.ToLower(name), strings.ToLower(name))
}

func androidManifest(name string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application
        android:allowBackup="true"
        android:icon="@mipmap/ic_launcher"
        android:label="@string/app_name"
        android:roundIcon="@mipmap/ic_launcher_round"
        android:supportsRtl="true"
        android:theme="@style/Theme.%s">
        <activity
            android:name=".MainActivity"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`, name)
}

func mainActivityKotlin(name string) string {
	return fmt.Sprintf(`package com.moduforge.%s

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity

class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
    }
}
`, strings.ToLower(name))
}

func mainActivityJava(name string) string {
	return fmt.Sprintf(`package com.moduforge.%s;

import android.os.Bundle;
import androidx.appcompat.app.AppCompatActivity;

public class MainActivity extends AppCompatActivity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
    }
}
`, strings.ToLower(name))
}

func mainLayout() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<androidx.constraintlayout.widget.ConstraintLayout
    xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:app="http://schemas.android.com/apk/res-auto"
    android:layout_width="match_parent"
    android:layout_height="match_parent">

    <TextView
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:text="@string/app_name"
        android:textSize="24sp"
        app:layout_constraintBottom_toBottomOf="parent"
        app:layout_constraintEnd_toEndOf="parent"
        app:layout_constraintStart_toStartOf="parent"
        app:layout_constraintTop_toTopOf="parent" />

</androidx.constraintlayout.widget.ConstraintLayout>
`
}

func stringsXML(name string) string {
	return fmt.Sprintf(`<resources>
    <string name="app_name">%s</string>
</resources>
`, name)
}

func themesXML() string {
	return `<resources>
    <style name="Theme.ModuForge" parent="Theme.MaterialComponents.DayNight.DarkActionBar">
        <item name="colorPrimary">#6366f1</item>
        <item name="colorPrimaryVariant">#4f46e5</item>
        <item name="colorOnPrimary">#ffffff</item>
        <item name="colorSecondary">#8b5cf6</item>
        <item name="colorSecondaryVariant">#7c3aed</item>
        <item name="colorOnSecondary">#ffffff</item>
    </style>
</resources>
`
}

// LogFn adapter for builder compatibility
var _ = log.Println
