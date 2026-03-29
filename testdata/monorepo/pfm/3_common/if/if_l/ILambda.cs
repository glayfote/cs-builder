using Pfm.Common.IfC;
using Pfm.Common.IfE;
using Pfm.Common.IfF;

namespace Pfm.Common.IfL;

/// <summary>if_c, if_e, if_f の 3 依存（IFoo 経由で IGamma も継承）。</summary>
public interface ILambda : IFoo, IEpsilon
{
    long Serial { get; }
}
