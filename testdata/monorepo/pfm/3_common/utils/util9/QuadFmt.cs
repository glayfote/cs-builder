using Pfm.Common.IfA;
using Pfm.Common.IfC;
using Pfm.Common.IfD;
using Pfm.Common.IfE;

namespace Pfm.Common.Utils.Util9;

/// <summary>if_a + if_c + if_d + if_e（4 依存）。</summary>
public static class QuadFmt
{
    public static string Line(IAlpha a, IGamma g, IDelta d, IEpsilon e) =>
        $"{a.Label}|{g.Name}|{d.Code}|{e.Active}";
}
