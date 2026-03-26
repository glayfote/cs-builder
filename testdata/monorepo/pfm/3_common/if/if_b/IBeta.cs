using Pfm.Common.IfA;

namespace Pfm.Common.IfB;

/// <summary>if_a に依存する第2インターフェース層。</summary>
public interface IBeta : IAlpha
{
    int Version { get; }
}
