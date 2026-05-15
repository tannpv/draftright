using System;
using System.Collections.Generic;
using System.Net;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;

namespace DraftRightWindows.Tests;

/// <summary>
/// Test double for <see cref="HttpMessageHandler"/>. Each test supplies a
/// <c>responder</c> that returns the response (or throws) for a given request,
/// optionally with a synchronous attempt counter so retry behavior is testable.
/// </summary>
internal sealed class TestHttpHandler : HttpMessageHandler
{
    private readonly Func<HttpRequestMessage, int, Task<HttpResponseMessage>> _responder;
    private int _calls;
    public List<HttpRequestMessage> Requests { get; } = new();
    public int CallCount => _calls;

    public TestHttpHandler(Func<HttpRequestMessage, int, Task<HttpResponseMessage>> responder)
    {
        _responder = responder;
    }

    /// <summary>Simple constant-response handler.</summary>
    public static TestHttpHandler Always(HttpStatusCode status, string body, string contentType = "application/json") =>
        new((req, _) => Task.FromResult(new HttpResponseMessage(status)
        {
            Content = new StringContent(body, System.Text.Encoding.UTF8, contentType)
        }));

    /// <summary>Returns a byte-payload response (e.g. installer body).</summary>
    public static TestHttpHandler Bytes(byte[] body, string contentType = "application/octet-stream") =>
        new((req, _) =>
        {
            var msg = new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new ByteArrayContent(body)
            };
            msg.Content.Headers.ContentType = new System.Net.Http.Headers.MediaTypeHeaderValue(contentType);
            msg.Content.Headers.ContentLength = body.Length;
            return Task.FromResult(msg);
        });

    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        Requests.Add(request);
        var callNumber = Interlocked.Increment(ref _calls);
        return _responder(request, callNumber);
    }
}
