package skills

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

// AndroidAppSkill generates a complete Android APP project for a Magisk/KernelSU/APatch module.
type AndroidAppSkill struct {
	projectPath string
	db          *sql.DB
	storage     storage.StorageAdapter
}

// NewAndroidAppSkillWithDB creates a new AndroidAppSkill with database support.
func NewAndroidAppSkillWithDB(projectPath string, db *sql.DB) *AndroidAppSkill {
	return &AndroidAppSkill{projectPath: projectPath, db: db}
}

// WithStorage sets the S3 storage adapter.
func (s *AndroidAppSkill) WithStorage(st storage.StorageAdapter) *AndroidAppSkill {
	s.storage = st
	return s
}

func (s *AndroidAppSkill) Name() string {
	return "android_app"
}

func (s *AndroidAppSkill) Description() string {
	return `Generate a complete Android APP project for a Magisk/KernelSU/APatch module.
Input: {"project_id": "...", "app_name": "MyModule", "package_name": "com.example.mymodule", "module_id": "mymodule", "description": "...", "features": ["settings_ui", "monitor", "dashboard"]}.
Generates Material Design 3 Kotlin project with module communication via SharedPreferences.
Output: list of generated files.`
}

func (s *AndroidAppSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	projectID, _ := input["project_id"].(string)
	appName, _ := input["app_name"].(string)
	packageName, _ := input["package_name"].(string)
	moduleID, _ := input["module_id"].(string)
	description, _ := input["description"].(string)
	featuresRaw, _ := input["features"].([]interface{})

	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}
	if appName == "" {
		appName = "ModuleApp"
	}
	if packageName == "" {
		packageName = "com.example.moduleapp"
	}
	if moduleID == "" {
		moduleID = "module"
	}
	if description == "" {
		description = appName + " companion application"
	}

	var features []string
	for _, f := range featuresRaw {
		if s, ok := f.(string); ok {
			features = append(features, s)
		}
	}
	if len(features) == 0 {
		features = []string{"settings_ui", "monitor"}
	}

	projectPath := ResolveProjectPath(s.db, s.projectPath, projectID)
	_ = filepath.Join(projectPath, "app") // reserved for future APK copy logic
	packagePath := strings.ReplaceAll(packageName, ".", "/")

	var generated []string

	files := map[string]string{
		"build.gradle.kts":                                        s.rootBuildGradle(),
		"settings.gradle.kts":                                     s.settingsGradle(appName),
		"gradle.properties":                                       s.gradleProperties(),
		"local.properties":                                        s.localProperties(),
		"gradle/wrapper/gradle-wrapper.properties":                s.gradleWrapperProperties(),
		"app/build.gradle.kts":                                    s.appBuildGradle(packageName),
		"app/src/main/AndroidManifest.xml":                        s.androidManifest(packageName, appName),
		fmt.Sprintf("app/src/main/java/%s/MainActivity.kt", packagePath): s.mainActivity(packageName, appName, moduleID, features),
		fmt.Sprintf("app/src/main/java/%s/ModuleService.kt", packagePath): s.moduleService(packageName, moduleID),
		"app/src/main/res/layout/activity_main.xml":               s.activityLayout(features),
		"app/src/main/res/values/strings.xml":                     s.stringsXML(appName),
		"app/src/main/res/values/colors.xml":                      s.colorsXML(),
		"app/src/main/res/values/themes.xml":                      s.themesXML(),
		"app/src/main/res/drawable/ic_launcher_foreground.xml":    s.launcherIcon(),
	}

	// Ensure customize.sh includes APK installation if not already present
	customizePath := filepath.Join(projectPath, "customize.sh")
	s.ensureAPKInstallInCustomize(customizePath, moduleID)

	for relPath, content := range files {
		fullPath := filepath.Join(projectPath, relPath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create dir %s: %w", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("write %s: %w", relPath, err)
		}
		generated = append(generated, relPath)
	}

	// Write files to S3 if storage is configured
	if s.storage != nil && s.db != nil {
		for relPath, content := range files {
			writeFileContent(ctx, s.storage, s.db, projectID, relPath, content)
		}
	}

	var log strings.Builder
	log.WriteString(fmt.Sprintf("Generated Android APP project: %s\n", appName))
	log.WriteString(fmt.Sprintf("Package: %s\n", packageName))
	log.WriteString(fmt.Sprintf("Module ID: %s\n", moduleID))
	log.WriteString(fmt.Sprintf("Description: %s\n", description))
	log.WriteString(fmt.Sprintf("Features: %s\n", strings.Join(features, ", ")))
	log.WriteString(fmt.Sprintf("Files generated: %d\n", len(generated)))
	log.WriteString("\nGenerated files:\n")
	for _, f := range generated {
		log.WriteString(fmt.Sprintf("  - %s\n", f))
	}
	log.WriteString("\nNext: use build_android_app to compile the APK")

	return log.String(), nil
}

func (s *AndroidAppSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		Core:      true, // Always expose to all model tiers
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}

// ── Gradle Build Files ──────────────────────────────────────────────

func (s *AndroidAppSkill) rootBuildGradle() string {
	return `// Top-level build file
plugins {
    id("com.android.application") version "8.2.2" apply false
    id("org.jetbrains.kotlin.android") version "1.9.22" apply false
}
`
}

func (s *AndroidAppSkill) settingsGradle(appName string) string {
	return fmt.Sprintf(`pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolution {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
}

rootProject.name = "%s"
include(":app")
`, appName)
}

func (s *AndroidAppSkill) gradleProperties() string {
	return `org.gradle.jvmargs=-Xmx2048m -Dfile.encoding=UTF-8
android.useAndroidX=true
kotlin.code.style=official
android.nonTransitiveRClass=true
`
}

func (s *AndroidAppSkill) localProperties() string {
	return `## This file must *NOT* be checked into Version Control Systems,
# as it contains information specific to your local configuration.
#
# Location of the Android SDK. This is only used by Gradle.
sdk.dir=/opt/android-sdk
`
}

func (s *AndroidAppSkill) gradleWrapperProperties() string {
	return `distributionBase=GRADLE_USER_HOME
distributionPath=wrapper/dists
distributionUrl=https\://services.gradle.org/distributions/gradle-8.5-bin.zip
networkTimeout=10000
validateDistributionUrl=true
zipStoreBase=GRADLE_USER_HOME
zipStorePath=wrapper/dists
`
}

func (s *AndroidAppSkill) appBuildGradle(packageName string) string {
	return fmt.Sprintf(`plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "%s"
    compileSdk = 34

    defaultConfig {
        applicationId = "%s"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        viewBinding = true
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("androidx.appcompat:appcompat:1.6.1")
    implementation("com.google.android.material:material:1.11.0")
    implementation("androidx.constraintlayout:constraintlayout:2.1.4")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.7.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-ktx:2.7.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.7.3")
}
`, packageName, packageName)
}

// ── Android Manifest ────────────────────────────────────────────────

func (s *AndroidAppSkill) androidManifest(packageName, appName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">

    <uses-permission android:name="android.permission.INTERNET" />

    <application
        android:allowBackup="true"
        android:icon="@mipmap/ic_launcher"
        android:label="@string/app_name"
        android:supportsRtl="true"
        android:theme="@style/Theme.%s">

        <activity
            android:name=".MainActivity"
            android:exported="true"
            android:theme="@style/Theme.%s">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>

        <service
            android:name=".ModuleService"
            android:exported="false" />

    </application>

</manifest>
`, strings.ReplaceAll(appName, " ", ""), strings.ReplaceAll(appName, " ", ""))
}

// ── Kotlin Source Files ─────────────────────────────────────────────

func (s *AndroidAppSkill) mainActivity(packageName, appName, moduleID string, features []string) string {
	hasMonitor := containsFeature(features, "monitor")
	hasDashboard := containsFeature(features, "dashboard")
	hasSettings := containsFeature(features, "settings_ui")

	monitorCode := ""
	if hasMonitor {
		monitorCode = `
        // Monitor section
        val monitorCard = findViewById<com.google.android.material.card.MaterialCardView>(R.id.monitorCard)
        val statusText = findViewById<android.widget.TextView>(R.id.statusText)
        val moduleStatus = prefs.getString("module_status", "unknown")
        statusText.text = when (moduleStatus) {
            "running" -> "● Running"
            "stopped" -> "● Stopped"
            "error" -> "● Error"
            else -> "● Unknown"
        }
        val statusColor = when (moduleStatus) {
            "running" -> com.google.android.material.color.MaterialColors.getColor(this, com.google.android.material.R.attr.colorPrimary, 0)
            "stopped" -> com.google.android.material.color.MaterialColors.getColor(this, com.google.android.material.R.attr.colorError, 0)
            else -> com.google.android.material.color.MaterialColors.getColor(this, com.google.android.material.R.attr.colorOnSurfaceVariant, 0)
        }
        statusText.setTextColor(statusColor)
        monitorCard.visibility = android.view.View.VISIBLE`
	}

	dashboardCode := ""
	if hasDashboard {
		dashboardCode = `
        // Dashboard section
        val dashboardCard = findViewById<com.google.android.material.card.MaterialCardView>(R.id.dashboardCard)
        val uptimeText = findViewById<android.widget.TextView>(R.id.uptimeText)
        val lastUpdateText = findViewById<android.widget.TextView>(R.id.lastUpdateText)
        val lastUpdate = prefs.getLong("last_update_time", 0L)
        if (lastUpdate > 0) {
            val sdf = java.text.SimpleDateFormat("yyyy-MM-dd HH:mm:ss", java.util.Locale.getDefault())
            lastUpdateText.text = sdf.format(java.util.Date(lastUpdate))
        }
        uptimeText.text = prefs.getString("uptime", "N/A")
        dashboardCard.visibility = android.view.View.VISIBLE`
	}

	settingsCode := ""
	if hasSettings {
		settingsCode = `
        // Settings section
        val settingsCard = findViewById<com.google.android.material.card.MaterialCardView>(R.id.settingsCard)
        val enableSwitch = findViewById<com.google.android.material.switchmaterial.SwitchMaterial>(R.id.enableSwitch)
        enableSwitch.isChecked = prefs.getBoolean("module_enabled", true)
        enableSwitch.setOnCheckedChangeListener { _, isChecked ->
            prefs.edit().putBoolean("module_enabled", isChecked).apply()
        }
        settingsCard.visibility = android.view.View.VISIBLE`
	}

	return fmt.Sprintf(`package %s

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import android.content.Context

class MainActivity : AppCompatActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        val prefs = getSharedPreferences("module_config", Context.MODE_WORLD_READABLE)

        val titleText = findViewById<android.widget.TextView>(R.id.titleText)
        titleText.text = "%s"
%s%s%s
    }
}
`, packageName, appName, monitorCode, dashboardCode, settingsCode)
}

func (s *AndroidAppSkill) moduleService(packageName, moduleID string) string {
	return fmt.Sprintf(`package %s

import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder

class ModuleService : Service() {

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val prefs = getSharedPreferences("module_config", Context.MODE_WORLD_READABLE)

        // Update module status
        prefs.edit()
            .putString("module_status", "running")
            .putLong("last_update_time", System.currentTimeMillis())
            .apply()

        // Module communication path:
        // /data/adb/modules/<module_id>/shared_prefs/ is shared between
        // the module shell scripts and this app via MODE_WORLD_READABLE.
        //
        // To read module data from shell:
        //   cat /data/adb/modules/%s/shared_prefs/module_config.xml
        //
        // To write data from shell (service.sh):
        //   settings put module_config key value

        return START_STICKY
    }

    override fun onDestroy() {
        super.onDestroy()
        val prefs = getSharedPreferences("module_config", Context.MODE_WORLD_READABLE)
        prefs.edit()
            .putString("module_status", "stopped")
            .putLong("last_update_time", System.currentTimeMillis())
            .apply()
    }
}
`, packageName, moduleID)
}

// ── Layout XML ──────────────────────────────────────────────────────

func (s *AndroidAppSkill) activityLayout(features []string) string {
	hasMonitor := containsFeature(features, "monitor")
	hasDashboard := containsFeature(features, "dashboard")
	hasSettings := containsFeature(features, "settings_ui")

	monitorSection := ""
	if hasMonitor {
		monitorSection = `
        <com.google.android.material.card.MaterialCardView
            android:id="@+id/monitorCard"
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:layout_marginTop="16dp"
            android:visibility="gone"
            app:cardCornerRadius="16dp"
            app:cardElevation="2dp"
            app:layout_constraintTop_toBottomOf="@id/titleText">

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:orientation="vertical"
                android:padding="16dp">

                <TextView
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:text="Module Status"
                    android:textAppearance="?attr/textAppearanceTitleMedium" />

                <TextView
                    android:id="@+id/statusText"
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:layout_marginTop="8dp"
                    android:text="● Unknown"
                    android:textAppearance="?attr/textAppearanceBodyLarge" />

            </LinearLayout>
        </com.google.android.material.card.MaterialCardView>`
	}

	dashboardSection := ""
	if hasDashboard {
		dashboardSection = `
        <com.google.android.material.card.MaterialCardView
            android:id="@+id/dashboardCard"
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:layout_marginTop="16dp"
            android:visibility="gone"
            app:cardCornerRadius="16dp"
            app:cardElevation="2dp"
            app:layout_constraintTop_toBottomOf="@id/monitorCard">

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:orientation="vertical"
                android:padding="16dp">

                <TextView
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:text="Dashboard"
                    android:textAppearance="?attr/textAppearanceTitleMedium" />

                <LinearLayout
                    android:layout_width="match_parent"
                    android:layout_height="wrap_content"
                    android:layout_marginTop="8dp"
                    android:orientation="horizontal">

                    <TextView
                        android:layout_width="0dp"
                        android:layout_height="wrap_content"
                        android:layout_weight="1"
                        android:text="Uptime:" />

                    <TextView
                        android:id="@+id/uptimeText"
                        android:layout_width="0dp"
                        android:layout_height="wrap_content"
                        android:layout_weight="2"
                        android:text="N/A" />

                </LinearLayout>

                <LinearLayout
                    android:layout_width="match_parent"
                    android:layout_height="wrap_content"
                    android:layout_marginTop="4dp"
                    android:orientation="horizontal">

                    <TextView
                        android:layout_width="0dp"
                        android:layout_height="wrap_content"
                        android:layout_weight="1"
                        android:text="Last Update:" />

                    <TextView
                        android:id="@+id/lastUpdateText"
                        android:layout_width="0dp"
                        android:layout_height="wrap_content"
                        android:layout_weight="2"
                        android:text="N/A" />

                </LinearLayout>

            </LinearLayout>
        </com.google.android.material.card.MaterialCardView>`
	}

	settingsSection := ""
	if hasSettings {
		settingsSection = `
        <com.google.android.material.card.MaterialCardView
            android:id="@+id/settingsCard"
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:layout_marginTop="16dp"
            android:visibility="gone"
            app:cardCornerRadius="16dp"
            app:cardElevation="2dp"
            app:layout_constraintTop_toBottomOf="@id/dashboardCard">

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:orientation="vertical"
                android:padding="16dp">

                <TextView
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:text="Settings"
                    android:textAppearance="?attr/textAppearanceTitleMedium" />

                <com.google.android.material.switchmaterial.SwitchMaterial
                    android:id="@+id/enableSwitch"
                    android:layout_width="match_parent"
                    android:layout_height="wrap_content"
                    android:layout_marginTop="8dp"
                    android:text="Enable Module"
                    android:textAppearance="?attr/textAppearanceBodyLarge" />

            </LinearLayout>
        </com.google.android.material.card.MaterialCardView>`
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<androidx.constraintlayout.widget.ConstraintLayout
    xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:app="http://schemas.android.com/apk/res-auto"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:padding="16dp"
    android:background="?attr/colorSurface">

    <TextView
        android:id="@+id/titleText"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:textAppearance="?attr/textAppearanceHeadlineMedium"
        android:textStyle="bold"
        app:layout_constraintTop_toTopOf="parent"
        app:layout_constraintStart_toStartOf="parent" />
%s%s%s

</androidx.constraintlayout.widget.ConstraintLayout>
`, monitorSection, dashboardSection, settingsSection)
}

// ── Resource XML ────────────────────────────────────────────────────

func (s *AndroidAppSkill) stringsXML(appName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">%s</string>
</resources>
`, appName)
}

func (s *AndroidAppSkill) colorsXML() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <color name="seed">#2196F3</color>
</resources>
`
}

func (s *AndroidAppSkill) themesXML() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <style name="Theme.ModuleApp" parent="Theme.Material3.DayNight.NoActionBar">
        <item name="colorSeed">@color/seed</item>
    </style>
</resources>
`
}

func (s *AndroidAppSkill) launcherIcon() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="108dp"
    android:height="108dp"
    android:viewportWidth="108"
    android:viewportHeight="108">
    <path
        android:fillColor="#2196F3"
        android:pathData="M54,54m-40,0a40,40 0,1 1,80 0a40,40 0,1 1,-80 0" />
    <path
        android:fillColor="#FFFFFF"
        android:pathData="M44,44h20v20h-20z" />
</vector>
`
}

// ── Helper ──────────────────────────────────────────────────────────

func containsFeature(features []string, name string) bool {
	for _, f := range features {
		if f == name {
			return true
		}
	}
	return false
}

// ensureAPKInstallInCustomize adds APK installation commands to customize.sh if not already present.
func (s *AndroidAppSkill) ensureAPKInstallInCustomize(customizePath, moduleID string) {
	data, err := os.ReadFile(customizePath)
	if err != nil {
		return
	}
	content := string(data)
	if strings.Contains(content, "app.apk") || strings.Contains(content, "companion.apk") {
		return
	}

	// Append APK installation block
	apkBlock := `
# ---- Install companion APK ----
if [ -f "$MODPATH/app/app.apk" ]; then
    ui_print "- Installing companion APK..."
    pm install -r "$MODPATH/app/app.apk" || ui_print "  ⚠️ APK install failed (non-fatal)"
fi
`
	os.WriteFile(customizePath, []byte(content+apkBlock), 0755)
}
