// Package buildorder は、選択された Visual Studio ソリューションに含まれる .csproj と
// その依存（ProjectReference / HintPath のアセンブリ参照）から有向グラフを構築し、
// dotnet build に渡すトポロジカル順序を計算する。
package buildorder
