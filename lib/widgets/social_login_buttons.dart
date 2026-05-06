import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/auth_service.dart';

class SocialLoginButtons extends StatefulWidget {
  const SocialLoginButtons({super.key});

  @override
  State<SocialLoginButtons> createState() => _SocialLoginButtonsState();
}

class _SocialLoginButtonsState extends State<SocialLoginButtons> {
  String? _loadingProvider;

  Future<void> _handleSocial(String provider, Future<void> Function() action) async {
    setState(() => _loadingProvider = provider);
    try {
      await action();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
        );
      }
    } finally {
      if (mounted) setState(() => _loadingProvider = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.read<AuthService>();

    return Column(
      children: [
        const _Divider(),
        const SizedBox(height: 16),
        _SocialButton(
          label: 'Continue with Google',
          icon: _googleIcon(),
          backgroundColor: Colors.white,
          textColor: Colors.black87,
          borderColor: Colors.grey.shade300,
          isLoading: _loadingProvider == 'google',
          onPressed: _loadingProvider != null ? null : () => _handleSocial('google', auth.signInWithGoogle),
        ),
        // Facebook + TikTok buttons hidden for the App Store submission until
        // their respective SDK credentials are wired up. Restore by enabling
        // the buttons below — both `signInWithFacebook` and `signInWithTikTok`
        // already exist in AuthService.
      ],
    );
  }

  Widget _googleIcon() {
    return SizedBox(
      width: 22, height: 22,
      child: CustomPaint(painter: _GoogleLogoPainter()),
    );
  }
}

class _Divider extends StatelessWidget {
  const _Divider();

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(child: Divider(color: Colors.grey.shade300)),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Text('or', style: TextStyle(color: Colors.grey.shade500, fontSize: 14)),
        ),
        Expanded(child: Divider(color: Colors.grey.shade300)),
      ],
    );
  }
}

class _SocialButton extends StatelessWidget {
  final String label;
  final Widget icon;
  final Color backgroundColor;
  final Color textColor;
  final Color? borderColor;
  final bool isLoading;
  final VoidCallback? onPressed;

  const _SocialButton({
    required this.label,
    required this.icon,
    required this.backgroundColor,
    required this.textColor,
    this.borderColor,
    this.isLoading = false,
    this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 48,
      width: double.infinity,
      child: OutlinedButton(
        onPressed: onPressed,
        style: OutlinedButton.styleFrom(
          backgroundColor: backgroundColor,
          side: BorderSide(color: borderColor ?? backgroundColor),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        ),
        child: isLoading
            ? SizedBox(
                height: 20, width: 20,
                child: CircularProgressIndicator(strokeWidth: 2, color: textColor),
              )
            : Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  icon,
                  const SizedBox(width: 12),
                  Text(label, style: TextStyle(color: textColor, fontSize: 15, fontWeight: FontWeight.w500)),
                ],
              ),
      ),
    );
  }
}

class _GoogleLogoPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final double w = size.width;
    final double h = size.height;
    final double cx = w / 2;
    final double cy = h / 2;
    final double r = w * 0.45;

    // Blue arc (top-right)
    final bluePaint = Paint()..color = const Color(0xFF4285F4)..style = PaintingStyle.stroke..strokeWidth = w * 0.18..strokeCap = StrokeCap.butt;
    canvas.drawArc(Rect.fromCircle(center: Offset(cx, cy), radius: r), -0.8, 1.8, false, bluePaint);

    // Green arc (bottom-right)
    final greenPaint = Paint()..color = const Color(0xFF34A853)..style = PaintingStyle.stroke..strokeWidth = w * 0.18..strokeCap = StrokeCap.butt;
    canvas.drawArc(Rect.fromCircle(center: Offset(cx, cy), radius: r), 1.0, 1.2, false, greenPaint);

    // Yellow arc (bottom-left)
    final yellowPaint = Paint()..color = const Color(0xFFFBBC05)..style = PaintingStyle.stroke..strokeWidth = w * 0.18..strokeCap = StrokeCap.butt;
    canvas.drawArc(Rect.fromCircle(center: Offset(cx, cy), radius: r), 2.2, 1.0, false, yellowPaint);

    // Red arc (top-left)
    final redPaint = Paint()..color = const Color(0xFFEA4335)..style = PaintingStyle.stroke..strokeWidth = w * 0.18..strokeCap = StrokeCap.butt;
    canvas.drawArc(Rect.fromCircle(center: Offset(cx, cy), radius: r), 3.2, 1.2, false, redPaint);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
