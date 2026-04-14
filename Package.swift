// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "DraftRight",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(
            name: "DraftRight",
            path: "DraftRight",
            exclude: ["Info.plist"],
            resources: [
                .copy("Resources/claude-icon.png"),
            ],
            swiftSettings: [
                .define("DEBUG", .when(configuration: .debug))
            ],
            linkerSettings: [
                .unsafeFlags([
                    "-Xlinker", "-sectcreate",
                    "-Xlinker", "__TEXT",
                    "-Xlinker", "__info_plist",
                    "-Xlinker", "DraftRight/Info.plist"
                ])
            ]
        )
    ]
)
