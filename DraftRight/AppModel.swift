import Foundation
import SwiftUI

@MainActor
final class AppModel: ObservableObject {
    @Published var apiKey: String = ""
    @Published var endpoint: String = "https://api.openai.com/v1/chat/completions"
    @Published var model: String = "gpt-4o-mini"
    @Published var temperature: Double = 0.3
    @Published var isRewriting: Bool = false
}
