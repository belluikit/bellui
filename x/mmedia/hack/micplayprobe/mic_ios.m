//go:build ios

// The probe's microphone capture. The session handling is belluikit/player's
// (sensor/sensor_ios.m, activateRecordSession / releaseRecordSession at Player
// main ba8188e), converted to ARC and otherwise unchanged, so the probe sees
// the session the way a Player app leaves it. The capture is an AVAudioEngine
// tap on the input node, as Player's CaptureAudio is; the tap counts buffers
// and sums the squares of channel 0 so Go can report the level per window.

#import <AVFoundation/AVFoundation.h>
#include <os/lock.h>
#include <stdlib.h>
#include <string.h>

static int gSessionRefs = 0;
static BOOL gRestorePlayback = NO;

static int activateRecordSession(void) {
	AVAudioSession *session = [AVAudioSession sharedInstance];
	@synchronized (session) {
		if (gSessionRefs == 0) {
			NSError *err = nil;
			BOOL playing = [session.category isEqualToString:AVAudioSessionCategoryPlayback];
			BOOL ok = playing
				? [session setCategory:AVAudioSessionCategoryPlayAndRecord
				           withOptions:AVAudioSessionCategoryOptionDefaultToSpeaker
				                 error:&err]
				: [session setCategory:AVAudioSessionCategoryRecord error:&err];
			if (!ok) {
				return 0;
			}
			if (![session setActive:YES error:&err]) {
				return 0;
			}
			gRestorePlayback = playing;
		}
		gSessionRefs++;
	}
	return 1;
}

static void releaseRecordSession(void) {
	AVAudioSession *session = [AVAudioSession sharedInstance];
	@synchronized (session) {
		if (gSessionRefs > 0 && --gSessionRefs == 0) {
			if (gRestorePlayback) {
				[session setCategory:AVAudioSessionCategoryPlayback error:nil];
				gRestorePlayback = NO;
				return;
			}
			[session setActive:NO
			       withOptions:AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation
			             error:nil];
		}
	}
}

static AVAudioEngine *gEngine = nil;
static os_unfair_lock gLock = OS_UNFAIR_LOCK_INIT;
static unsigned long long gBufs = 0, gFrames = 0;
static double gSumSq = 0;
static int gConfigChanges = 0;
static id gConfigObserver = nil;

int micPermission(void) {
	dispatch_semaphore_t done = dispatch_semaphore_create(0);
	__block BOOL granted = NO;
	[[AVAudioSession sharedInstance] requestRecordPermission:^(BOOL ok) {
		granted = ok;
		dispatch_semaphore_signal(done);
	}];
	dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
	return granted ? 1 : 0;
}

const char *micStart(void) {
	if (!activateRecordSession()) {
		return strdup("activateRecordSession failed");
	}
	AVAudioEngine *engine = [[AVAudioEngine alloc] init];
	AVAudioInputNode *input = engine.inputNode;
	AVAudioFormat *fmt = [input outputFormatForBus:0];
	if (fmt.sampleRate <= 0.0 || fmt.channelCount == 0) {
		releaseRecordSession();
		return strdup("no usable input hardware");
	}
	[input installTapOnBus:0
	            bufferSize:(AVAudioFrameCount)(fmt.sampleRate / 5.0)
	                format:fmt
	                 block:^(AVAudioPCMBuffer *buf, AVAudioTime *when) {
		if (buf == nil || buf.floatChannelData == NULL) {
			return;
		}
		const float *x = buf.floatChannelData[0];
		double s = 0;
		for (AVAudioFrameCount i = 0; i < buf.frameLength; i++) {
			s += (double)x[i] * (double)x[i];
		}
		os_unfair_lock_lock(&gLock);
		gBufs++;
		gFrames += buf.frameLength;
		gSumSq += s;
		os_unfair_lock_unlock(&gLock);
	}];
	gConfigObserver = [[NSNotificationCenter defaultCenter]
		addObserverForName:AVAudioEngineConfigurationChangeNotification
		            object:engine
		             queue:nil
		        usingBlock:^(NSNotification *n) {
		os_unfair_lock_lock(&gLock);
		gConfigChanges++;
		os_unfair_lock_unlock(&gLock);
	}];
	NSError *err = nil;
	if (![engine startAndReturnError:&err]) {
		[input removeTapOnBus:0];
		releaseRecordSession();
		return strdup([[err description] UTF8String]);
	}
	gEngine = engine;
	return NULL;
}

void micStop(void) {
	if (gEngine == nil) {
		return;
	}
	[gEngine.inputNode removeTapOnBus:0];
	[gEngine stop];
	gEngine = nil;
	if (gConfigObserver != nil) {
		[[NSNotificationCenter defaultCenter] removeObserver:gConfigObserver];
		gConfigObserver = nil;
	}
	releaseRecordSession();
}

void micStats(unsigned long long *bufs, unsigned long long *frames, double *sumSq, int *configChanges, int *running) {
	os_unfair_lock_lock(&gLock);
	*bufs = gBufs;
	*frames = gFrames;
	*sumSq = gSumSq;
	*configChanges = gConfigChanges;
	os_unfair_lock_unlock(&gLock);
	*running = (gEngine != nil && gEngine.isRunning) ? 1 : 0;
}

// sessionCategory returns the session's category name without the
// AVAudioSessionCategory prefix, malloc'd.
const char *sessionCategory(void) {
	NSString *c = [AVAudioSession sharedInstance].category;
	c = [c stringByReplacingOccurrencesOfString:@"AVAudioSessionCategory" withString:@""];
	return strdup(c.UTF8String);
}

// sessionOutputs returns the current route's output port types, comma
// separated, malloc'd: Speaker or Receiver (the earpiece) on a phone.
const char *sessionOutputs(void) {
	NSMutableArray<NSString *> *names = [NSMutableArray array];
	for (AVAudioSessionPortDescription *p in [AVAudioSession sharedInstance].currentRoute.outputs) {
		[names addObject:p.portType];
	}
	return strdup([names componentsJoinedByString:@","].UTF8String);
}
