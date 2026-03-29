import Foundation

struct ChatMessage: Codable {
    let role: String
    let content: String
}

struct ChatRequest: Codable {
    let model: String
    let messages: [ChatMessage]
    let temperature: Double
    let max_tokens: Int
}

struct ChatResponse: Codable {
    struct Choice: Codable {
        let message: ChatMessage
    }
    let choices: [Choice]
}

final class OpenAIClient {
    func rewrite(
        text: String,
        tone: Tone,
        settings: SharedSettings,
        completion: @escaping (Result<String, Error>) -> Void
    ) {
        guard let url = URL(string: settings.endpoint) else {
            completion(.failure(NSError(domain: "OpenAIClient", code: -1,
                userInfo: [NSLocalizedDescriptionKey: "Invalid endpoint URL"])))
            return
        }

        let inputText = String(text.prefix(3000))
        let messages = [
            ChatMessage(role: "system", content: tone.systemPrompt(targetLanguage: settings.translateLanguage)),
            ChatMessage(role: "user", content: inputText),
        ]

        let body = ChatRequest(model: settings.model, messages: messages,
                               temperature: settings.temperature, max_tokens: 1024)

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.timeoutInterval = 15

        let apiKey = settings.apiKey
        if !apiKey.isEmpty {
            request.addValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }

        do {
            request.httpBody = try JSONEncoder().encode(body)
        } catch {
            completion(.failure(error))
            return
        }

        URLSession.shared.dataTask(with: request) { data, response, error in
            if let error = error {
                completion(.failure(error))
                return
            }
            guard let data = data else {
                completion(.failure(NSError(domain: "OpenAIClient", code: -2,
                    userInfo: [NSLocalizedDescriptionKey: "No data returned"])))
                return
            }
            if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode >= 400 {
                let bodyText = String(data: data, encoding: .utf8) ?? "Unknown error"
                completion(.failure(NSError(domain: "OpenAIClient", code: httpResponse.statusCode,
                    userInfo: [NSLocalizedDescriptionKey: "HTTP \(httpResponse.statusCode): \(bodyText)"])))
                return
            }
            do {
                let decoded = try JSONDecoder().decode(ChatResponse.self, from: data)
                guard let first = decoded.choices.first else {
                    completion(.failure(NSError(domain: "OpenAIClient", code: -3,
                        userInfo: [NSLocalizedDescriptionKey: "No response from AI"])))
                    return
                }
                completion(.success(first.message.content.trimmingCharacters(in: .whitespacesAndNewlines)))
            } catch {
                completion(.failure(error))
            }
        }.resume()
    }
}
