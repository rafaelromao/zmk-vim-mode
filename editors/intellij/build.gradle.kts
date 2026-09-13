plugins {
    // The Kotlin compiler must be able to read the metadata in the IDE's own
    // jars: IntelliJ 2026.2 ships Kotlin 2.4, and an older compiler fails with
    // hundreds of "incompatible version of Kotlin" errors that look like the
    // standard library has vanished. Match it to your IDE.
    kotlin("jvm") version "2.4.0"
    id("org.jetbrains.intellij.platform") version "2.18.1"
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

        // IdeaVim from its installed directory rather than the Marketplace:
        // it is already on disk, it is guaranteed to be the version actually
        // running, and it needs no network -- which matters behind a proxy
        // that intercepts TLS, since the JDK has its own truststore and will
        // refuse the handshake the rest of the system accepts.
        localPlugin(providers.gradleProperty("ideaVimPath").get())
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
