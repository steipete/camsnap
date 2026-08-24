#import "bridge.h"

#import <AVFoundation/AVFoundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <Foundation/Foundation.h>
#import <ImageIO/ImageIO.h>

#include <stdlib.h>
#include <string.h>

static void AVFSetError(char **error_out, NSString *message) {
    if (error_out == NULL) {
        return;
    }

    const char *utf8 = message.UTF8String;
    *error_out = strdup(utf8 != NULL ? utf8 : "unknown AVFoundation error");
}

static NSArray<AVCaptureDeviceType> *AVFDeviceTypes(void) {
    return @[
        AVCaptureDeviceTypeBuiltInWideAngleCamera,
        AVCaptureDeviceTypeExternal,
        AVCaptureDeviceTypeContinuityCamera,
    ];
}

static AVCaptureDeviceDiscoverySession *AVFDiscoverySession(void) {
    return [AVCaptureDeviceDiscoverySession
        discoverySessionWithDeviceTypes:AVFDeviceTypes()
                           mediaType:AVMediaTypeVideo
                            position:AVCaptureDevicePositionUnspecified];
}

int avf_list_devices(AVFDevice **devices_out, size_t *count_out, char **error_out) {
    @autoreleasepool {
        if (devices_out == NULL || count_out == NULL) {
            AVFSetError(error_out, @"invalid device-list output pointers");
            return 0;
        }

        *devices_out = NULL;
        *count_out = 0;

        @try {
            NSArray<AVCaptureDevice *> *devices = AVFDiscoverySession().devices;
            AVCaptureDevice *default_device = [AVCaptureDevice defaultDeviceWithMediaType:AVMediaTypeVideo];
            size_t count = devices.count;
            if (count == 0) {
                return 1;
            }

            AVFDevice *result = calloc(count, sizeof(AVFDevice));
            if (result == NULL) {
                AVFSetError(error_out, @"allocate device list");
                return 0;
            }

            for (size_t i = 0; i < count; i++) {
                AVCaptureDevice *device = devices[i];
                result[i].unique_id = strdup(device.uniqueID.UTF8String ?: "");
                result[i].name = strdup(device.localizedName.UTF8String ?: "");
                result[i].is_default = default_device != nil &&
                    [device.uniqueID isEqualToString:default_device.uniqueID];

                if (result[i].unique_id == NULL || result[i].name == NULL) {
                    avf_free_devices(result, count);
                    AVFSetError(error_out, @"copy device metadata");
                    return 0;
                }
            }

            *devices_out = result;
            *count_out = count;
            return 1;
        } @catch (NSException *exception) {
            AVFSetError(error_out, [NSString stringWithFormat:@"device discovery: %@", exception.reason]);
            return 0;
        }
    }
}

void avf_free_devices(AVFDevice *devices, size_t count) {
    if (devices == NULL) {
        return;
    }

    for (size_t i = 0; i < count; i++) {
        free(devices[i].unique_id);
        free(devices[i].name);
    }
    free(devices);
}

int avf_authorization_status(void) {
    return (int)[AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeVideo];
}

int avf_request_access(unsigned long long token, char **error_out) {
    @autoreleasepool {
        @try {
            [AVCaptureDevice requestAccessForMediaType:AVMediaTypeVideo
                                    completionHandler:^(BOOL granted) {
                goAVFAccessResult(token, granted ? 1 : 0);
            }];
            return 1;
        } @catch (NSException *exception) {
            AVFSetError(error_out, [NSString stringWithFormat:@"request camera access: %@", exception.reason]);
            return 0;
        }
    }
}

@interface AVFFrameReceiver : NSObject <AVCaptureVideoDataOutputSampleBufferDelegate>

@property(nonatomic, readonly) dispatch_semaphore_t semaphore;
@property(nonatomic, readonly, nullable) NSString *failure;

- (instancetype)initWithOutputPath:(NSString *)output_path warmup:(NSTimeInterval)warmup;

@end

@implementation AVFFrameReceiver {
    NSString *_output_path;
    CFAbsoluteTime _ready_at;
    BOOL _finished;
    NSString *_failure;
}

- (instancetype)initWithOutputPath:(NSString *)output_path warmup:(NSTimeInterval)warmup {
    self = [super init];
    if (self != nil) {
        _output_path = [output_path copy];
        _ready_at = CFAbsoluteTimeGetCurrent() + warmup;
        _semaphore = dispatch_semaphore_create(0);
    }
    return self;
}

- (NSString *)failure {
    return _failure;
}

- (void)finishWithFailure:(nullable NSString *)failure {
    _failure = [failure copy];
    dispatch_semaphore_signal(_semaphore);
}

- (void)captureOutput:(AVCaptureOutput *)output
    didOutputSampleBuffer:(CMSampleBufferRef)sample_buffer
           fromConnection:(AVCaptureConnection *)connection {
    (void)output;
    (void)connection;

    if (_finished || CFAbsoluteTimeGetCurrent() < _ready_at) {
        return;
    }
    _finished = YES;

    CVImageBufferRef image_buffer = CMSampleBufferGetImageBuffer(sample_buffer);
    if (image_buffer == NULL) {
        [self finishWithFailure:@"sample buffer did not contain an image"];
        return;
    }

    CVReturn lock_result = CVPixelBufferLockBaseAddress(image_buffer, kCVPixelBufferLock_ReadOnly);
    if (lock_result != kCVReturnSuccess) {
        [self finishWithFailure:[NSString stringWithFormat:@"lock pixel buffer: %d", lock_result]];
        return;
    }

    void *base_address = CVPixelBufferGetBaseAddress(image_buffer);
    size_t width = CVPixelBufferGetWidth(image_buffer);
    size_t height = CVPixelBufferGetHeight(image_buffer);
    size_t bytes_per_row = CVPixelBufferGetBytesPerRow(image_buffer);

    CGColorSpaceRef color_space = CGColorSpaceCreateDeviceRGB();
    CGBitmapInfo bitmap_info = kCGBitmapByteOrder32Little | kCGImageAlphaPremultipliedFirst;
    CGContextRef bitmap_context = CGBitmapContextCreate(
        base_address,
        width,
        height,
        8,
        bytes_per_row,
        color_space,
        bitmap_info
    );
    CGColorSpaceRelease(color_space);

    if (bitmap_context == NULL) {
        CVPixelBufferUnlockBaseAddress(image_buffer, kCVPixelBufferLock_ReadOnly);
        [self finishWithFailure:@"create bitmap context"];
        return;
    }

    CGImageRef image = CGBitmapContextCreateImage(bitmap_context);
    CGContextRelease(bitmap_context);
    CVPixelBufferUnlockBaseAddress(image_buffer, kCVPixelBufferLock_ReadOnly);

    if (image == NULL) {
        [self finishWithFailure:@"create image from pixel buffer"];
        return;
    }

    NSURL *output_url = [NSURL fileURLWithPath:_output_path];
    CGImageDestinationRef destination = CGImageDestinationCreateWithURL(
        (__bridge CFURLRef)output_url,
        CFSTR("public.jpeg"),
        1,
        NULL
    );
    if (destination == NULL) {
        CGImageRelease(image);
        [self finishWithFailure:@"create JPEG destination"];
        return;
    }

    NSDictionary *properties = @{(__bridge NSString *)kCGImageDestinationLossyCompressionQuality: @0.9};
    CGImageDestinationAddImage(destination, image, (__bridge CFDictionaryRef)properties);
    BOOL finalized = CGImageDestinationFinalize(destination);
    CFRelease(destination);
    CGImageRelease(image);

    if (!finalized) {
        [self finishWithFailure:@"finalize JPEG output"];
        return;
    }

    [self finishWithFailure:nil];
}

@end

@interface AVFStreamSession : NSObject <AVCaptureVideoDataOutputSampleBufferDelegate>

@property(nonatomic, readonly) AVCaptureSession *session;
@property(nonatomic, readonly) dispatch_semaphore_t semaphore;

- (instancetype)initWithSession:(AVCaptureSession *)session output:(AVCaptureVideoDataOutput *)output;
- (void)stop;

@end

@implementation AVFStreamSession {
    AVCaptureVideoDataOutput *_output;
    dispatch_queue_t _queue;
    BOOL _received_frame;
}

- (instancetype)initWithSession:(AVCaptureSession *)session output:(AVCaptureVideoDataOutput *)output {
    self = [super init];
    if (self != nil) {
        _session = session;
        _output = output;
        _queue = dispatch_queue_create("com.steipete.camsnap.avf.stream", DISPATCH_QUEUE_SERIAL);
        _semaphore = dispatch_semaphore_create(0);
        [_output setSampleBufferDelegate:self queue:_queue];
    }
    return self;
}

- (void)captureOutput:(AVCaptureOutput *)output
    didOutputSampleBuffer:(CMSampleBufferRef)sample_buffer
           fromConnection:(AVCaptureConnection *)connection {
    (void)output;
    (void)sample_buffer;
    (void)connection;

    if (!_received_frame) {
        _received_frame = YES;
        dispatch_semaphore_signal(_semaphore);
    }
}

- (void)stop {
    [_session stopRunning];
    [_output setSampleBufferDelegate:nil queue:NULL];
    dispatch_sync(_queue, ^{});
}

@end

static AVCaptureDevice *AVFFindDevice(const char *device_id) {
    if (device_id == NULL || device_id[0] == '\0') {
        return [AVCaptureDevice defaultDeviceWithMediaType:AVMediaTypeVideo];
    }

    NSString *wanted = [NSString stringWithUTF8String:device_id];
    for (AVCaptureDevice *device in AVFDiscoverySession().devices) {
        if ([device.uniqueID isEqualToString:wanted]) {
            return device;
        }
    }
    return nil;
}

static BOOL AVFCheckAuthorization(char **error_out) {
    AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeVideo];
    if (status == AVAuthorizationStatusAuthorized) {
        return YES;
    }

    NSString *status_name = @"unknown";
    switch (status) {
        case AVAuthorizationStatusNotDetermined:
            status_name = @"notDetermined";
            break;
        case AVAuthorizationStatusRestricted:
            status_name = @"restricted";
            break;
        case AVAuthorizationStatusDenied:
            status_name = @"denied";
            break;
        case AVAuthorizationStatusAuthorized:
            break;
    }
    AVFSetError(error_out, [NSString stringWithFormat:@"camera access is %@", status_name]);
    return NO;
}

void *avf_open_session(const char *device_id, char **error_out) {
    @autoreleasepool {
        if (!AVFCheckAuthorization(error_out)) {
            return NULL;
        }

        @try {
            AVCaptureDevice *device = AVFFindDevice(device_id);
            if (device == nil) {
                AVFSetError(error_out, device_id == NULL || device_id[0] == '\0'
                    ? @"no default video capture device"
                    : @"video capture device not found");
                return NULL;
            }

            NSError *input_error = nil;
            AVCaptureDeviceInput *input = [AVCaptureDeviceInput deviceInputWithDevice:device error:&input_error];
            if (input == nil) {
                AVFSetError(error_out, [NSString stringWithFormat:@"create device input: %@", input_error.localizedDescription]);
                return NULL;
            }

            AVCaptureSession *session = [[AVCaptureSession alloc] init];
            AVCaptureVideoDataOutput *output = [[AVCaptureVideoDataOutput alloc] init];
            output.alwaysDiscardsLateVideoFrames = YES;
            output.videoSettings = @{
                (__bridge NSString *)kCVPixelBufferPixelFormatTypeKey: @(kCVPixelFormatType_32BGRA),
            };

            [session beginConfiguration];
            if (![session canAddInput:input]) {
                [session commitConfiguration];
                AVFSetError(error_out, @"capture session rejected device input");
                return NULL;
            }
            [session addInput:input];
            if (![session canAddOutput:output]) {
                [session commitConfiguration];
                AVFSetError(error_out, @"capture session rejected video output");
                return NULL;
            }
            [session addOutput:output];
            [session commitConfiguration];

            AVFStreamSession *stream = [[AVFStreamSession alloc] initWithSession:session output:output];
            [session startRunning];
            long wait_result = dispatch_semaphore_wait(
                stream.semaphore,
                dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC)
            );
            if (wait_result != 0) {
                [stream stop];
                AVFSetError(error_out, @"timed out waiting for a video frame");
                return NULL;
            }
            return (__bridge_retained void *)stream;
        } @catch (NSException *exception) {
            AVFSetError(error_out, [NSString stringWithFormat:@"open capture session: %@", exception.reason]);
            return NULL;
        }
    }
}

int avf_close_session(void *session, char **error_out) {
    @autoreleasepool {
        if (session == NULL) {
            return 1;
        }

        AVFStreamSession *stream = (__bridge_transfer AVFStreamSession *)session;
        @try {
            [stream stop];
            return 1;
        } @catch (NSException *exception) {
            AVFSetError(error_out, [NSString stringWithFormat:@"close capture session: %@", exception.reason]);
            return 0;
        }
    }
}

int avf_capture_frame(const char *device_id, double warmup_seconds, const char *out_path, char **error_out) {
    @autoreleasepool {
        if (!AVFCheckAuthorization(error_out)) {
            return 0;
        }

        @try {
            AVCaptureDevice *device = AVFFindDevice(device_id);
            if (device == nil) {
                AVFSetError(error_out, device_id == NULL || device_id[0] == '\0'
                    ? @"no default video capture device"
                    : @"video capture device not found");
                return 0;
            }

            NSError *input_error = nil;
            AVCaptureDeviceInput *input = [AVCaptureDeviceInput deviceInputWithDevice:device error:&input_error];
            if (input == nil) {
                AVFSetError(error_out, [NSString stringWithFormat:@"create device input: %@", input_error.localizedDescription]);
                return 0;
            }

            AVCaptureSession *session = [[AVCaptureSession alloc] init];
            AVCaptureVideoDataOutput *output = [[AVCaptureVideoDataOutput alloc] init];
            output.alwaysDiscardsLateVideoFrames = YES;
            output.videoSettings = @{
                (__bridge NSString *)kCVPixelBufferPixelFormatTypeKey: @(kCVPixelFormatType_32BGRA),
            };

            [session beginConfiguration];
            if (![session canAddInput:input]) {
                [session commitConfiguration];
                AVFSetError(error_out, @"capture session rejected device input");
                return 0;
            }
            [session addInput:input];
            if (![session canAddOutput:output]) {
                [session commitConfiguration];
                AVFSetError(error_out, @"capture session rejected video output");
                return 0;
            }
            [session addOutput:output];
            [session commitConfiguration];

            NSString *output_path = [NSString stringWithUTF8String:out_path];
            AVFFrameReceiver *receiver = [[AVFFrameReceiver alloc]
                initWithOutputPath:output_path
                            warmup:warmup_seconds];
            dispatch_queue_t capture_queue = dispatch_queue_create("com.steipete.camsnap.avf.capture", DISPATCH_QUEUE_SERIAL);
            [output setSampleBufferDelegate:receiver queue:capture_queue];

            [session startRunning];
            NSTimeInterval timeout_seconds = warmup_seconds + 10.0;
            long wait_result = dispatch_semaphore_wait(
                receiver.semaphore,
                dispatch_time(DISPATCH_TIME_NOW, (int64_t)(timeout_seconds * NSEC_PER_SEC))
            );
            [session stopRunning];
            [output setSampleBufferDelegate:nil queue:NULL];
            dispatch_sync(capture_queue, ^{});

            if (wait_result != 0) {
                AVFSetError(error_out, @"timed out waiting for a video frame");
                return 0;
            }
            if (receiver.failure != nil) {
                AVFSetError(error_out, receiver.failure);
                return 0;
            }
            return 1;
        } @catch (NSException *exception) {
            AVFSetError(error_out, [NSString stringWithFormat:@"capture frame: %@", exception.reason]);
            return 0;
        }
    }
}
