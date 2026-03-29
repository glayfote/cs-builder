using Pfm.Common.IfA;
using Pfm.Common.IfC;

namespace Pfm.Common.Utils.Util6;

/// <summary>if_a + if_c（2 依存）。</summary>
public static class AlphaGammaFmt
{
    public static string Line(IAlpha a, IGamma g) => $"{a.Label}:{g.Name}";
}
