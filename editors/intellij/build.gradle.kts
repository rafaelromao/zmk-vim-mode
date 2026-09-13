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

kotlin {
    jvmToolchain(21)
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
