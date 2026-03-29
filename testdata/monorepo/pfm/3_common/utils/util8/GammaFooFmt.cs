using Pfm.Common.IfC;
using Pfm.Common.IfF;

namespace Pfm.Common.Utils.Util8;

/// <summary>if_c + if_f（2 依存）。</summary>
public static class GammaFooFmt
{
    public static string Line(IGamma g, IFoo f) => $"{g.Name}:{f.Priority}";
}
