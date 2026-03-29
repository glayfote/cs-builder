using Pfm.Common.IfC;
using Pfm.Common.IfD;
using Pfm.Common.IfE;

namespace Pfm.Common.IfJ;

/// <summary>if_c, if_d, if_e の 3 依存。</summary>
public interface IJota : IGamma, IDelta, IEpsilon
{
    string Kind { get; }
}
