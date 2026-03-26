using Pfm.Common.IfB;
using Pfm.Common.Utils.Util1;

namespace Pfm.Driver.PkgA;

file record DemoBeta(string Label, int Version) : IBeta;

internal static class Program
{
    private static void Main()
    {
        var beta = new DemoBeta("pkg-a", 1);
        Console.WriteLine(beta.FormatLabel());
        Console.WriteLine(PkgAEntry.Run());
    }
}
