using Pfm.Common.IfA;
using Pfm.Common.IfC;
using Pfm.Common.IfD;
using Pfm.Common.IfE;
using Pfm.Common.Utils.Util3;
using Pfm.Common.Utils.Util4;
using Pfm.Common.Utils.Util5;

namespace Pfm.Driver.PkgG1;

file record Demo(string Label, string Name, int Code, bool Active) : IAlpha, IGamma, IDelta, IEpsilon;

internal static class Program
{
    private static void Main()
    {
        var d = new Demo("pkg-g-1", "g", 7, true);
        Console.WriteLine(TokenBuilder.Join(",", [d.Label, d.Name]));
        Console.WriteLine(SmallMath.Clamp(d.Code, 0, 99));
        Console.WriteLine(TextSlice.Mid(d.Label, 0, 6));
    }
}
