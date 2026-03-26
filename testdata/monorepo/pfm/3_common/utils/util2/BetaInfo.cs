using Pfm.Common.IfB;

namespace Pfm.Common.Utils.Util2;

public static class BetaInfo
{
    public static string Describe(IBeta beta) => $"{beta.Label} v{beta.Version}";
}
