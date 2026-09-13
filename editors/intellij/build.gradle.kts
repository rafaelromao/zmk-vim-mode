plugins {
    // If either of these fails to resolve, the version is simply out of date:
    // check plugins.gradle.org for the current one. Nothing here depends on a
    // particular version beyond supporting your Gradle and IDE.
    kotlin("jvm") version "2.2.20"
    id("org.jetbrains.intellij.platform") version "2.9.0"
}

group = "dev.rafaelromao"
version = "0.1.0"

repositories {
    mavenCentral()
    intellijPlatform { defaultRepositories() }
}

dependencies {
    intellijPlatform {
        // Builds against the IDE you already have, so the platform itself is
        // never downloaded. Point it at your own installation; on Linux that
        // is the directory the Toolbox unpacked.
        local(providers.gradleProperty("platformPath").get())

        // IdeaVim, from the Marketplace. Set the version to the one you have:
        // Settings -> Plugins -> Installed -> IdeaVim.
        plugin("IdeaVIM", providers.gradleProperty("ideaVimVersion").get())
    }
}

// Target Java 21 bytecode with whatever JDK runs Gradle, rather than asking
// for a JDK 21 toolchain: requiring one means either having that exact version
// installed or letting Gradle download it. The IDE runs on 21 or newer, so 21
// is the safe floor.
kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_21)
    }
}

java {
    sourceCompatibility = JavaVersion.VERSION_21
    targetCompatibility = JavaVersion.VERSION_21
}

intellijPlatform {
    pluginConfiguration {
        ideaVersion {
            // Widen if your IDE is older; the build fails loudly if it is
            // outside the range.
            sinceBuild = providers.gradleProperty("sinceBuild").get()
        }
    }
}
