using Pfm.Common.IfA;

namespace Pfm.Driver.PkgI3;

file record Demo(string Label) : IAlpha;

internal static class Program
{
    private static void Main()
    {
        var d = new Demo("pkg-i-3");
        Console.WriteLine(d.Label);
    }
}
