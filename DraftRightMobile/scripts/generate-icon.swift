#!/usr/bin/env swift

import AppKit

func generateIcon(size: Int) -> NSImage {
    let img = NSImage(size: NSSize(width: size, height: size))
    img.lockFocus()

    let s = CGFloat(size)
    let rect = NSRect(x: 0, y: 0, width: s, height: s)

    // Blue gradient background (rounded rect for adaptive icon)
    let path = NSBezierPath(roundedRect: rect, xRadius: s * 0.2, yRadius: s * 0.2)
    let gradient = NSGradient(colors: [
        NSColor(red: 0.15, green: 0.40, blue: 0.95, alpha: 1.0),
        NSColor(red: 0.10, green: 0.25, blue: 0.80, alpha: 1.0)
    ])!
    gradient.draw(in: path, angle: -45)

    // Draw pencil icon (SF Symbol)
    let symbolConfig = NSImage.SymbolConfiguration(pointSize: s * 0.4, weight: .medium)
    if let symbol = NSImage(systemSymbolName: "pencil.and.outline", accessibilityDescription: nil)?
        .withSymbolConfiguration(symbolConfig) {
        let symbolSize = symbol.size
        let x = (s - symbolSize.width) / 2
        let y = (s - symbolSize.height) / 2 - s * 0.02
        symbol.draw(in: NSRect(x: x, y: y, width: symbolSize.width, height: symbolSize.height),
                    from: .zero, operation: .sourceOver, fraction: 1.0)
        // Tint white
        NSColor.white.setFill()
        NSRect(x: x, y: y, width: symbolSize.width, height: symbolSize.height).fill(using: .sourceAtop)
    }

    img.unlockFocus()
    return img
}

// Generate 1024x1024 source icon
let icon = generateIcon(size: 1024)
guard let tiff = icon.tiffRepresentation,
      let rep = NSBitmapImageRep(data: tiff),
      let png = rep.representation(using: .png, properties: [:]) else {
    print("Failed to generate icon")
    exit(1)
}

let outputPath = "/opt/openAi/DraftRight/DraftRightMobile/assets/icon.png"
let dir = (outputPath as NSString).deletingLastPathComponent
try? FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true)
try! png.write(to: URL(fileURLWithPath: outputPath))
print("Icon generated at \(outputPath)")
