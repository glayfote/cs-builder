namespace Pfm.Common.Utils.Util5;

/// <summary>if 依存なしのユーティリティ。</summary>
public static class TokenBuilder
{
    public static string Join(string sep, IEnumerable<string> parts) => string.Join(sep, parts);
}
