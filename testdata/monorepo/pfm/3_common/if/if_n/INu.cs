using Pfm.Common.IfC;
using Pfm.Common.IfD;
using Pfm.Common.IfF;

namespace Pfm.Common.IfN;

/// <summary>if_c, if_d, if_f の 3 依存。</summary>
public interface INu : IGamma, IDelta, IFoo
{
    float Weight { get; }
}
