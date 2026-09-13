// Package intellijplugin embeds the IntelliJ plugin's sources so the daemon
// binary can build and install it without a checkout.
//
// The Kotlin has to be compiled, which no amount of embedding avoids -- but
// the Gradle wrapper is here too, so the only thing the user needs installed
// is the IDE itself, whose bundled JVM runs the build.
package intellijplugin

import "embed"

// Files are everything needed to build the plugin. gradle.properties is
// deliberately absent: the installer generates it from the IDE it found.
//
//go:embed build.gradle.kts settings.gradle.kts gradlew gradlew.bat
//go:embed gradle/wrapper/gradle-wrapper.jar gradle/wrapper/gradle-wrapper.properties
//go:embed src
var Files embed.FS
