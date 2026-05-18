// swift-tools-version:5.9
import PackageDescription

// Pure-Swift package containing the testable core of DraftRight's iOS
// keyboard extension. Lives outside the Xcode workspace so `swift test`
// runs headlessly on the developer's Mac (no iOS simulator boot per
// test cycle). The actual iOS keyboard target consumes these sources
// by file reference inside Xcode — see DraftRightMobile/ios/DraftRightKeyboard/.
//
// Target macOS 13 + iOS 13 means the same code compiles for both
// the `swift test` host (Mac) and the iOS extension build.
let package = Package(
    name: "DraftRightKeyboardCore",
    platforms: [
        .macOS(.v13),
        .iOS(.v13),
    ],
    products: [
        .library(name: "DraftRightKeyboardCore", targets: ["DraftRightKeyboardCore"]),
    ],
    targets: [
        .target(
            name: "DraftRightKeyboardCore",
            path: "Sources/DraftRightKeyboardCore"
        ),
        .testTarget(
            name: "DraftRightKeyboardCoreTests",
            dependencies: ["DraftRightKeyboardCore"],
            path: "Tests/DraftRightKeyboardCoreTests"
        ),
    ]
)
