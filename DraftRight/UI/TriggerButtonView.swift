import SwiftUI

/// Simple pencil button that appears near selected text.
/// Clicking it opens the rewrite panel.
struct TriggerButtonView: View {
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            Image(systemName: "pencil.and.outline")
                .font(.system(size: 14, weight: .medium))
                .foregroundColor(.white)
                .frame(width: 28, height: 28)
                .background(Color.accentColor, in: Circle())
                .shadow(radius: 2)
        }
        .buttonStyle(.plain)
    }
}
