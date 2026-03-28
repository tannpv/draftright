import Foundation

enum DiffKind {
    case equal
    case deleted
    case inserted
}

struct DiffToken {
    let text: String
    let kind: DiffKind
}

enum WordDiff {
    static func diff(old: String, new: String) -> (oldTokens: [DiffToken], newTokens: [DiffToken]) {
        let oldWords = tokenize(old)
        let newWords = tokenize(new)

        let lcs = longestCommonSubsequence(oldWords, newWords)
        var oldTokens: [DiffToken] = []
        var newTokens: [DiffToken] = []

        var oi = 0, ni = 0, li = 0
        while oi < oldWords.count || ni < newWords.count {
            if li < lcs.count {
                while oi < oldWords.count && oldWords[oi] != lcs[li] {
                    oldTokens.append(DiffToken(text: oldWords[oi], kind: .deleted))
                    oi += 1
                }
                while ni < newWords.count && newWords[ni] != lcs[li] {
                    newTokens.append(DiffToken(text: newWords[ni], kind: .inserted))
                    ni += 1
                }
                if li < lcs.count {
                    oldTokens.append(DiffToken(text: lcs[li], kind: .equal))
                    newTokens.append(DiffToken(text: lcs[li], kind: .equal))
                    oi += 1
                    ni += 1
                    li += 1
                }
            } else {
                while oi < oldWords.count {
                    oldTokens.append(DiffToken(text: oldWords[oi], kind: .deleted))
                    oi += 1
                }
                while ni < newWords.count {
                    newTokens.append(DiffToken(text: newWords[ni], kind: .inserted))
                    ni += 1
                }
            }
        }

        return (oldTokens, newTokens)
    }

    private static func tokenize(_ text: String) -> [String] {
        var tokens: [String] = []
        var current = ""
        for char in text {
            if char.isWhitespace {
                if !current.isEmpty {
                    tokens.append(current)
                    current = ""
                }
                tokens.append(String(char))
            } else {
                current.append(char)
            }
        }
        if !current.isEmpty {
            tokens.append(current)
        }
        return tokens
    }

    private static func longestCommonSubsequence(_ a: [String], _ b: [String]) -> [String] {
        let m = a.count, n = b.count
        var dp = Array(repeating: Array(repeating: 0, count: n + 1), count: m + 1)

        for i in 1...m {
            for j in 1...n {
                if a[i - 1] == b[j - 1] {
                    dp[i][j] = dp[i - 1][j - 1] + 1
                } else {
                    dp[i][j] = max(dp[i - 1][j], dp[i][j - 1])
                }
            }
        }

        var result: [String] = []
        var i = m, j = n
        while i > 0 && j > 0 {
            if a[i - 1] == b[j - 1] {
                result.append(a[i - 1])
                i -= 1
                j -= 1
            } else if dp[i - 1][j] > dp[i][j - 1] {
                i -= 1
            } else {
                j -= 1
            }
        }

        return result.reversed()
    }
}
