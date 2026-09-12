plugins {
    kotlin("jvm") version "2.0.21"
    id("org.jetbrains.intellij.platform") version "2.1.0"
}

group = "dev.rafaelromao"
version = "0.1.0"

repositories {
    mavenCentral()
    intellijPlatform { defaultRepositories() }
}

dependencies {
    intellijPlatform {
        // Builds against the IDE you already have, so nothing is downloaded
        // for the platform itself. Point it at your own installation; on Linux
        // that is the directory the Toolbox unpacked.
        local(providers.gradleProperty("platformPath"))

        // IdeaVim, from the Marketplace. Set the version to the one you have
        // installed: Settings -> Plugins -> Installed -> IdeaVim.
        plugin("IdeaVIM", providers.gradleProperty("ideaVimVersion"))
    }
}

kotlin {
    jvmToolchain(21)
}

intellijPlatform {
    pluginConfiguration {
        ideaVersion {
            // Widen if your IDE is older; buildPlugin fails loudly if it is
            // outside the range.
            sinceBuild = providers.gradleProperty("sinceBuild")
        }
    }
}
