using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.Tests;

/// <summary>
/// Unit tests for <see cref="ApiClient.ExtractServerMessage"/> — the parser
/// that turns an error response body into the one-line message the UI shows.
///
/// BUG-44: after the Go backend cutover, error bodies switched from NestJS's
/// {"message":"…"} to {"error":"…","code":"…","request_id":"…"}. The old parser
/// only read "message", so Go errors fell through to the raw JSON and users
/// saw e.g. {"error":"Account disabled",...} verbatim on Google sign-in.
/// These tests lock in support for every shape both backends emit.
/// </summary>
public class ApiClientErrorMessageTests
{
    [Fact]
    public void Extracts_Go_Error_Field()
    {
        // The exact body from BUG-44's logcat.
        var body = "{\"error\":\"Account disabled\",\"code\":\"invalid-token\",\"request_id\":\"7408d581-ca06-457f-bf55-37c357b0d53b\"}";
        Assert.Equal("Account disabled", ApiClient.ExtractServerMessage(body));
    }

    [Fact]
    public void Extracts_Nest_String_Message()
    {
        var body = "{\"message\":\"Invalid credentials\",\"statusCode\":401}";
        Assert.Equal("Invalid credentials", ApiClient.ExtractServerMessage(body));
    }

    [Fact]
    public void Extracts_And_Joins_Nest_Array_Message()
    {
        var body = "{\"message\":[\"email must be an email\",\"password should not be empty\"]}";
        Assert.Equal("email must be an email; password should not be empty", ApiClient.ExtractServerMessage(body));
    }

    [Fact]
    public void Prefers_Message_Over_Error_When_Both_Present()
    {
        // If a body carries both, the human-readable "message" wins.
        var body = "{\"message\":\"Something friendly\",\"error\":\"Bad Request\"}";
        Assert.Equal("Something friendly", ApiClient.ExtractServerMessage(body));
    }

    [Fact]
    public void Falls_Back_To_Raw_Body_When_Not_Json()
    {
        var body = "upstream 502 bad gateway";
        Assert.Equal(body, ApiClient.ExtractServerMessage(body));
    }

    [Fact]
    public void Falls_Back_To_Raw_Body_When_No_Known_Field()
    {
        var body = "{\"detail\":\"nope\",\"foo\":42}";
        Assert.Equal(body, ApiClient.ExtractServerMessage(body));
    }

    [Theory]
    [InlineData("")]
    [InlineData("   ")]
    public void Handles_Empty_Body(string body)
    {
        Assert.Equal(body, ApiClient.ExtractServerMessage(body));
    }
}
