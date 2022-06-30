package midi

import (
	"fmt"
	"testing"
)

func TestGenerateMidiFile(t *testing.T) {
	srcFile := "/Users/cenggang/lnmp_env/www/gowww/starify_go/submodule/rtc_algorithm/music.mp3"
	dstFile := "/Users/cenggang/lnmp_env/www/gowww/starify_go/submodule/rtc_algorithm/music.txt"
	LrcFile := "/Users/cenggang/lnmp_env/www/gowww/starify_go/submodule/rtc_algorithm/music.lrc"
	ret := GenerateMidiFile(srcFile, dstFile, LrcFile)
	if !ret {
		fmt.Println("GenerateMidiFile file failed! src file", srcFile)
	}
}
