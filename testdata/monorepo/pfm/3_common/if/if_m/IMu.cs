using Pfm.Common.IfA;
using Pfm.Common.IfD;
using Pfm.Common.IfE;

namespace Pfm.Common.IfM;

/// <summary>if_a, if_d, if_e の 3 依存。</summary>
public interface IMu : IAlpha, IDelta, IEpsilon
{
    char Marker { get; }
}
