using Pfm.Common.IfA;
using Pfm.Common.IfC;
using Pfm.Common.IfD;

namespace Pfm.Common.IfI;

/// <summary>if_a, if_c, if_d の 3 依存。</summary>
public interface IIota : IAlpha, IGamma, IDelta
{
    int Index { get; }
}
