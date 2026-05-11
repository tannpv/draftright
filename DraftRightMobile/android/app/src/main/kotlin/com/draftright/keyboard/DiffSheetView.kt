package com.draftright.keyboard

import android.content.Context
import android.graphics.Color
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView

class DiffSheetView(
    context: Context,
    original: String,
    rewritten: String,
    private val onReplace: (String) -> Unit,
    private val onCopy: (String) -> Unit,
    private val onCancel: () -> Unit
) : LinearLayout(context) {

    init {
        orientation = VERTICAL
        setBackgroundColor(resolveAttrColor(android.R.attr.colorBackground))
        setPadding(dpToPx(12), dpToPx(8), dpToPx(12), dpToPx(12))

        // Drag handle
        val handle = View(context).apply {
            setBackgroundColor(Color.LTGRAY)
            layoutParams = LayoutParams(dpToPx(36), dpToPx(4)).apply {
                gravity = Gravity.CENTER_HORIZONTAL
                bottomMargin = dpToPx(8)
            }
        }
        addView(handle)

        // Diff row
        val diffRow = LinearLayout(context).apply {
            orientation = HORIZONTAL
            layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, 0, 1f)
        }

        // Original column
        val leftCol = LinearLayout(context).apply {
            orientation = VERTICAL
            layoutParams = LayoutParams(0, LayoutParams.MATCH_PARENT, 1f).apply {
                marginEnd = dpToPx(4)
            }
            setBackgroundColor(Color.parseColor("#08FF0000"))
            setPadding(dpToPx(6), dpToPx(6), dpToPx(6), dpToPx(6))
        }
        leftCol.addView(TextView(context).apply {
            text = "Original"
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 11f)
            setTextColor(Color.GRAY)
        })
        val origScroll = ScrollView(context).apply {
            layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT)
        }
        origScroll.addView(TextView(context).apply {
            text = original
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 14f)
            setPadding(0, dpToPx(4), 0, 0)
        })
        leftCol.addView(origScroll)
        diffRow.addView(leftCol)

        // Rewritten column
        val rightCol = LinearLayout(context).apply {
            orientation = VERTICAL
            layoutParams = LayoutParams(0, LayoutParams.MATCH_PARENT, 1f).apply {
                marginStart = dpToPx(4)
            }
            setBackgroundColor(Color.parseColor("#0800FF00"))
            setPadding(dpToPx(6), dpToPx(6), dpToPx(6), dpToPx(6))
        }
        rightCol.addView(TextView(context).apply {
            text = "Rewritten"
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 11f)
            setTextColor(Color.GRAY)
        })
        val rewriteScroll = ScrollView(context).apply {
            layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT)
        }
        rewriteScroll.addView(TextView(context).apply {
            text = rewritten
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 14f)
            setPadding(0, dpToPx(4), 0, 0)
        })
        rightCol.addView(rewriteScroll)
        diffRow.addView(rightCol)

        addView(diffRow)

        // Button row
        val buttonRow = LinearLayout(context).apply {
            orientation = HORIZONTAL
            gravity = Gravity.END
            layoutParams = LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT).apply {
                topMargin = dpToPx(8)
            }
        }

        buttonRow.addView(Button(context).apply {
            text = "Cancel"
            setOnClickListener { onCancel() }
            layoutParams = LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT).apply {
                marginEnd = dpToPx(8)
            }
        })
        buttonRow.addView(Button(context).apply {
            text = "Copy"
            setOnClickListener { onCopy(rewritten) }
            layoutParams = LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT).apply {
                marginEnd = dpToPx(8)
            }
        })
        buttonRow.addView(Button(context).apply {
            text = "Replace"
            setOnClickListener { onReplace(rewritten) }
        })

        addView(buttonRow)
    }

    private fun dpToPx(dp: Int): Int =
        TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, dp.toFloat(), resources.displayMetrics).toInt()

    private fun resolveAttrColor(attr: Int): Int {
        val tv = TypedValue()
        return if (context.theme.resolveAttribute(attr, tv, true)) tv.data else Color.WHITE
    }
}
