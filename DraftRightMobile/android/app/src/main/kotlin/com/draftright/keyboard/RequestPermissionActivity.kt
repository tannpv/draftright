package com.draftright.keyboard

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Bundle

class RequestPermissionActivity : Activity() {
    companion object {
        private const val EXTRA_PERMISSION = "permission"
        fun launch(context: Context, permission: String) {
            context.startActivity(Intent(context, RequestPermissionActivity::class.java).apply {
                putExtra(EXTRA_PERMISSION, permission)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            })
        }
    }
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val permission = intent.getStringExtra(EXTRA_PERMISSION) ?: return finish()
        requestPermissions(arrayOf(permission), 1)
    }

    // Result reported back to the IME via SharedSettings (VOICE-011): the IME
    // may not still be the foreground window by the time this fires, so we
    // can't call back into it directly — the flag survives to be read on the
    // IME's next onWindowShown instead.
    override fun onRequestPermissionsResult(code: Int, perms: Array<out String>, results: IntArray) {
        val granted = results.isNotEmpty() && results[0] == PackageManager.PERMISSION_GRANTED
        SharedSettings(this).voicePermissionResult =
            if (granted) SharedSettings.VOICE_PERMISSION_GRANTED else SharedSettings.VOICE_PERMISSION_DENIED
        finish()
    }
}
