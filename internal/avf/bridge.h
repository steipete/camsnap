#ifndef CAMSNAP_AVF_BRIDGE_H
#define CAMSNAP_AVF_BRIDGE_H

#include <stddef.h>

typedef struct {
    char *unique_id;
    char *name;
    int is_default;
} AVFDevice;

int avf_list_devices(AVFDevice **devices_out, size_t *count_out, char **error_out);
void avf_free_devices(AVFDevice *devices, size_t count);

int avf_authorization_status(void);
int avf_request_access(unsigned long long token, char **error_out);

void *avf_open_session(const char *device_id, char **error_out);
int avf_close_session(void *session, char **error_out);
int avf_capture_frame(const char *device_id, double warmup_seconds, const char *out_path, char **error_out);

extern void goAVFAccessResult(unsigned long long token, int granted);

#endif
