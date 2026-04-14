import SwiftUI

struct SettingsView: View {
    var body: some View {
        TabView {
            GeneralSettingsTab()
                .tabItem { Label("General", systemImage: "gearshape") }
            RewriteSettingsTab()
                .tabItem { Label("Rewrite", systemImage: "pencil.and.outline") }
            AccountSettingsTab()
                .tabItem { Label("Account", systemImage: "person.crop.circle") }
            AdvancedSettingsTab()
                .tabItem { Label("Advanced", systemImage: "slider.horizontal.3") }
        }
        .frame(width: 480, height: 420)
    }
}
