using Pfm.Common.IfA;
using Pfm.Common.IfC;
using Pfm.Common.IfD;
using Pfm.Common.Utils.Util3;

namespace Pfm.Driver.PkgF4;

file record Demo(string Label, string Name, int Code) : IAlpha, IGamma, IDelta;

internal static class Program
{
    private static void Main()
    {
        var d = new Demo("pkg-f-4", "gamma", 42);
        Console.WriteLine(TextSlice.Mid(d.Label, 0, 6));
        Console.WriteLine($"{d.Name}:{d.Code}");
    }
}
