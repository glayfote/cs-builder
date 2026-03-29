using Pfm.Common.IfC;

namespace Pfm.Common.IfF;

/// <summary>if_c（IGamma）に依存するインターフェース。</summary>
public interface IFoo : IGamma
{
    int Priority { get; }
}
