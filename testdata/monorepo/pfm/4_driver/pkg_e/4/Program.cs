using Pfm.Common.IfA;
using Pfm.Common.IfC;

namespace Pfm.Driver.PkgE4;

file record Demo(string Label, string Name) : IAlpha, IGamma;

internal static class Program
{
    private static void Main()
    {
        var d = new Demo("pkg-e-4", "gamma");
        Console.WriteLine($"{d.Label}:{d.Name}");
    }
}
