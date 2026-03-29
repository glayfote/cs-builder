namespace Pfm.Common.Utils.Util4;

/// <summary>if 依存なしのユーティリティ。</summary>
public static class SmallMath
{
    public static int Clamp(int v, int lo, int hi) => v < lo ? lo : v > hi ? hi : v;
}
