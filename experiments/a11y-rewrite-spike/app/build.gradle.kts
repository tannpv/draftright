plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.draftright.spike"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.draftright.spike"
        minSdk = 26
        // targetSdk 33 on purpose: at 34+ every foreground service needs a
        // declared type + special-use justification. This is a throwaway
        // feasibility probe, so 33 keeps the FGS boilerplate out of the way
        // while still exercising the real overlay + AccessibilityService APIs.
        targetSdk = 33
        versionCode = 1
        versionName = "0.1-spike"
    }

    buildTypes {
        getByName("debug") {
            isMinifyEnabled = false
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
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
}
