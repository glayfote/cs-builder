using Pfm.Common.IfB;
using Pfm.Common.Utils.Util2;
using Pfm.Driver.PkgA;

namespace Pfm.Driver.PkgB;

file record DemoBeta(string Label, int Version) : IBeta;

internal static class Program
{
    private static void Main()
    {
        Console.WriteLine(PkgAEntry.Run());
        Console.WriteLine(BetaInfo.Describe(new DemoBeta("pkg-b", 2)));
    }
}
