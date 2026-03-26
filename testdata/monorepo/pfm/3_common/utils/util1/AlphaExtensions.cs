using Pfm.Common.IfA;

namespace Pfm.Common.Utils.Util1;

public static class AlphaExtensions
{
    public static string FormatLabel(this IAlpha alpha) => $"[{alpha.Label}]";
}
