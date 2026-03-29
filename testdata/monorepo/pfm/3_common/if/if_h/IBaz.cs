using Pfm.Common.IfA;
using Pfm.Common.IfE;

namespace Pfm.Common.IfH;

/// <summary>if_a（IAlpha）と if_e（IEpsilon）に依存するインターフェース。</summary>
public interface IBaz : IAlpha, IEpsilon
{
    string Scope { get; }
}
