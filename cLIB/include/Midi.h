//
//  Midi.h
//  pitch
//
//  Created by 杨沛 on 2022/1/27.
//

#ifndef __MIDI_H__
#define __MIDI_H__
#include <stdio.h>

#ifdef __cplusplus
extern "C"{
#endif

#define BB_API __attribute__((visibility("default")))

BB_API int GenerateMidiFile(char* Mp3FileName, char* TargetFileName, char* lrcFileName);

#ifdef __cplusplus
}
#endif
#endif /* Midi_h */
