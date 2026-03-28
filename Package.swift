// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "DraftRight",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(
            name: "DraftRight",
            path: "DraftRight",
            exclude: ["Info.plist"]
        )
    ]
)
