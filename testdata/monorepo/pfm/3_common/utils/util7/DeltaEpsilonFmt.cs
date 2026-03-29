using Pfm.Common.IfD;
using Pfm.Common.IfE;

namespace Pfm.Common.Utils.Util7;

/// <summary>if_d + if_e（2 依存）。</summary>
public static class DeltaEpsilonFmt
{
    public static string Line(IDelta d, IEpsilon e) => $"{d.Code}:{e.Active}";
}
