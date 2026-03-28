#!/usr/bin/env swift

import AppKit

// Generate DraftRight app icon: pencil + checkmark on blue gradient
func generateIcon(size: Int) -> NSImage {
    let img = NSImage(size: NSSize(width: size, height: size))
    img.lockFocus()

    let rect = NSRect(x: 0, y: 0, width: size, height: size)
    let s = CGFloat(size)

    // Rounded rect background with blue gradient
    let path = NSBezierPath(roundedRect: rect.insetBy(dx: s * 0.04, dy: s * 0.04),
                            xRadius: s * 0.22, yRadius: s * 0.22)
    let gradient = NSGradient(colors: [
        NSColor(red: 0.18, green: 0.45, blue: 0.95, alpha: 1.0),
        NSColor(red: 0.12, green: 0.30, blue: 0.80, alpha: 1.0)
    ])!
    gradient.draw(in: path, angle: -45)

    // Draw SF Symbol "pencil.and.outline" as the icon
    let symbolConfig = NSImage.SymbolConfiguration(pointSize: s * 0.45, weight: .medium)
    if let symbol = NSImage(systemSymbolName: "pencil.and.outline", accessibilityDescription: nil)?
        .withSymbolConfiguration(symbolConfig) {
        let symbolSize = symbol.size
        let x = (s - symbolSize.width) / 2
        let y = (s - symbolSize.height) / 2
        symbol.draw(in: NSRect(x: x, y: y, width: symbolSize.width, height: symbolSize.height),
                    from: .zero, operation: .sourceOver, fraction: 1.0)

        // Tint white by drawing over with blend
        NSColor.white.setFill()
        NSRect(x: x, y: y, width: symbolSize.width, height: symbolSize.height).fill(using: .sourceAtop)
    }

    img.unlockFocus()
    return img
}

// Create iconset directory
let iconsetPath = "/tmp/DraftRight.iconset"
let fm = FileManager.default
try? fm.removeItem(atPath: iconsetPath)
try fm.createDirectory(atPath: iconsetPath, withIntermediateDirectories: true)

// Required icon sizes for .icns
let sizes: [(name: String, size: Int)] = [
    ("icon_16x16", 16),
    ("icon_16x16@2x", 32),
    ("icon_32x32", 32),
    ("icon_32x32@2x", 64),
    ("icon_128x128", 128),
    ("icon_128x128@2x", 256),
    ("icon_256x256", 256),
    ("icon_256x256@2x", 512),
    ("icon_512x512", 512),
    ("icon_512x512@2x", 1024),
]

for entry in sizes {
    let image = generateIcon(size: entry.size)
    guard let tiff = image.tiffRepresentation,
          let rep = NSBitmapImageRep(data: tiff),
          let png = rep.representation(using: .png, properties: [:]) else {
        print("Failed to generate \(entry.name)")
        continue
    }
    let filePath = "\(iconsetPath)/\(entry.name).png"
    try png.write(to: URL(fileURLWithPath: filePath))
    print("Generated \(entry.name) (\(entry.size)x\(entry.size))")
}

print("Iconset created at \(iconsetPath)")
print("Run: iconutil -c icns \(iconsetPath) -o /tmp/DraftRight.icns")
