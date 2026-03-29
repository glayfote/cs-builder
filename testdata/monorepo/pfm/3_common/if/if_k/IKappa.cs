using Pfm.Common.IfA;
using Pfm.Common.IfB;
using Pfm.Common.IfC;

namespace Pfm.Common.IfK;

/// <summary>if_a, if_b, if_c の 3 依存。</summary>
public interface IKappa : IAlpha, IBeta, IGamma
{
    byte Rank { get; }
}
