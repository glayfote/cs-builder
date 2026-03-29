using Pfm.Common.IfC;
using Pfm.Common.IfD;

namespace Pfm.Common.IfG;

/// <summary>if_c と if_d の両方に依存するインターフェース。</summary>
public interface IBar : IGamma, IDelta
{
    string Tag { get; }
}
