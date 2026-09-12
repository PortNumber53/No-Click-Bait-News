import 'package:intl/intl.dart';

String formatTimeAgo(
  DateTime publishedAt, {
  DateTime? now,
  bool useCalendarDateAfterWeek = false,
}) {
  final difference = (now ?? DateTime.now()).difference(publishedAt);
  if (difference.isNegative || difference.inMinutes < 1) return 'Just now';
  if (difference.inMinutes < 60) return '${difference.inMinutes}m ago';
  if (difference.inHours < 24) return '${difference.inHours}h ago';
  if (difference.inDays < 7) return '${difference.inDays}d ago';
  if (useCalendarDateAfterWeek) return DateFormat.MMMd().format(publishedAt);
  if (difference.inDays < 30) return '${difference.inDays}d ago';
  return '${difference.inDays ~/ 30}mo ago';
}
