using Pfm.Common.IfA;

namespace Pfm.Driver.PkgI2;

file record Demo(string Label) : IAlpha;

internal static class Program
{
    private static void Main()
    {
        var d = new Demo("pkg-i-2");
        Console.WriteLine(d.Label);
    }
}
