package com.draftright.keyboard

import android.app.Activity
import android.content.Context
import android.content.Intent
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
    override fun onRequestPermissionsResult(code: Int, perms: Array<out String>, results: IntArray) = finish()
}
