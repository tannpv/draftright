namespace DraftRightWindows.Helpers;

/// <summary>
/// URL normalization utilities (mirrors macOS String.strippingTrailingSlash).
/// </summary>
public static class UrlHelper
{
    /// <summary>
    /// Strips one or more trailing '/' characters from a URL string.
    /// </summary>
    public static string StripTrailingSlash(this string url) => url.TrimEnd('/');
}
