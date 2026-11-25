//go:build ios

#include <stdlib.h>
#include <string.h>
#import <AVFoundation/AVFoundation.h>

// mmediaSetPlaybackSession sets the shared audio session's category for
// playback and activates it. It returns NULL, or a malloc'd description of
// the error, which the caller frees.
//
// The category is Playback, unless the app is recording from the microphone:
// Playback has no input, so switching to it would silence a running capture
// (#1188). A session in Record moves to PlayAndRecord, keeping its options and
// adding DefaultToSpeaker (without it, PlayAndRecord plays from the earpiece),
// which is what belluikit/player does when a capture starts during playback. A
// session already in PlayAndRecord keeps the category and options the app
// chose.
//
// The check and the change run under @synchronized on the session, the lock
// Player's capture code takes for its own check-and-set, so a capture starting
// at the same moment sees either the old category or the new one.
char *mmediaSetPlaybackSession(void) {
	NSError *err = nil;
	AVAudioSession *s = [AVAudioSession sharedInstance];
	@synchronized (s) {
		NSString *cat = s.category;
		BOOL ok = YES;
		if ([cat isEqualToString:AVAudioSessionCategoryRecord]) {
			ok = [s setCategory:AVAudioSessionCategoryPlayAndRecord
			        withOptions:s.categoryOptions | AVAudioSessionCategoryOptionDefaultToSpeaker
			              error:&err];
		} else if (![cat isEqualToString:AVAudioSessionCategoryPlayAndRecord]) {
			ok = [s setCategory:AVAudioSessionCategoryPlayback error:&err];
		}
		if (!ok) {
			return strdup([[err description] UTF8String]);
		}
		if (![s setActive:YES error:&err]) {
			return strdup([[err description] UTF8String]);
		}
	}
	return NULL;
}
