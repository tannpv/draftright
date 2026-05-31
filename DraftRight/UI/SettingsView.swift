import SwiftUI

struct SettingsView: View {
    var body: some View {
        TabView {
            GeneralSettingsTab()
                .tabItem { Label("General", systemImage: "gearshape") }
            RewriteSettingsTab()
                .tabItem { Label("Rewrite", systemImage: "pencil.and.outline") }
            TriggerSettingsTab()
                .tabItem { Label("Trigger", systemImage: "cursorarrow.click") }
            AccountSettingsTab()
                .tabItem { Label("Account", systemImage: "person.crop.circle") }
            SubscriptionSettingsTab()
                .tabItem { Label("Subscription", systemImage: "creditcard.and.123") }
            AdvancedSettingsTab()
                .tabItem { Label("Advanced", systemImage: "slider.horizontal.3") }
        }
        .frame(width: 560, height: 560)
    }
}
