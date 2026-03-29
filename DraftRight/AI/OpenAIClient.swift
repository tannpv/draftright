import Foundation

struct AIMessage: Codable {
    let role: String
    let content: String
}

struct ChatRequest: Codable {
    let model: String
    let messages: [AIMessage]
    let temperature: Double
    let max_tokens: Int
}

struct ChatResponse: Codable {
    struct Choice: Codable {
        let message: AIMessage
    }
    let choices: [Choice]
}

enum OpenAIClientError: LocalizedError {
    case missingAPIKey
    case invalidEndpoint
    case emptyResponse
    case httpError(Int, String)
    case timeout

    var errorDescription: String? {
        switch self {
        case .missingAPIKey: return "API key not set. Open Settings to configure."
        case .invalidEndpoint: return "Invalid API endpoint URL."
        case .emptyResponse: return "No text returned from API."
        case .httpError(let code, let body): return "HTTP \(code): \(body)"
        case .timeout: return "Request timed out after 10 seconds."
        }
    }
}

final class OpenAIClient {
    private let session: URLSession

    init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 10
        self.session = URLSession(configuration: config)
    }

    func rewrite(text: String, tone: Tone, apiKey: String, endpoint: String, model: String, temperature: Double, targetLanguage: String = "English") async throws -> String {
        guard let url = URL(string: endpoint) else { throw OpenAIClientError.invalidEndpoint }

        let inputText = String(text.prefix(3000))
        let messages = [
            AIMessage(role: "system", content: tone.systemPrompt(targetLanguage: targetLanguage)),
            AIMessage(role: "user", content: inputText)
        ]

        let body = ChatRequest(model: model, messages: messages, temperature: temperature, max_tokens: 1024)
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        if !apiKey.isEmpty {
            request.addValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }
        request.httpBody = try JSONEncoder().encode(body)

        let (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: request)
        } catch let error as URLError where error.code == .timedOut {
            throw OpenAIClientError.timeout
        }

        if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
            let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw OpenAIClientError.httpError(httpResponse.statusCode, bodyText)
        }

        let decoded = try JSONDecoder().decode(ChatResponse.self, from: data)
        guard let first = decoded.choices.first else { throw OpenAIClientError.emptyResponse }
        return first.message.content.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
