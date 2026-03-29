using Pfm.Common.IfA;
using Pfm.Common.IfC;
using Pfm.Common.IfD;
using Pfm.Common.IfE;
using Pfm.Common.IfF;
using Pfm.Common.Utils.Util3;
using Pfm.Common.Utils.Util4;

namespace Pfm.Driver.PkgK1;

file record Demo(string Label, string Name, int Code, bool Active, int Priority) : IAlpha, IDelta, IEpsilon, IFoo;

internal static class Program
{
    private static void Main()
    {
        var d = new Demo("pkg-k-1", "g", 3, true, 9);
        Console.WriteLine(TextSlice.Mid(d.Label, 0, 6));
        Console.WriteLine(SmallMath.Clamp(d.Priority, 0, 10));
    }
}
