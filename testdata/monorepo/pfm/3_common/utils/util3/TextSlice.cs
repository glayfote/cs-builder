namespace Pfm.Common.Utils.Util3;

/// <summary>if 依存なしのユーティリティ。</summary>
public static class TextSlice
{
    public static string Mid(string s, int start, int len) =>
        s.Length <= start ? "" : s.Substring(start, Math.Min(len, s.Length - start));
}
