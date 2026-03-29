using Pfm.Common.IfA;

namespace Pfm.Driver.PkgI4;

file record Demo(string Label) : IAlpha;

internal static class Program
{
    private static void Main()
    {
        var d = new Demo("pkg-i-4");
        Console.WriteLine(d.Label);
    }
}
