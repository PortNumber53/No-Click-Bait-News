import 'dart:async';
import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import '../models/article.dart';

/// User-scoped cache for articles the backend has granted for the next seven
/// days. The backend-provided expiry remains authoritative, including when a
/// rewrite refreshes while the article is open.
class ArticleAccessCache {
  static const _keyPrefix = 'article_access_v1:';
  static const _maximumRetention = Duration(days: 7);
  static final Map<String, _CachedArticle> _memory = {};
  static SharedPreferences? _preferences;

  static String _key(String userId, String articleId) =>
      '$_keyPrefix$userId:$articleId';

  static Article? peek(String userId, String articleId) {
    final key = _key(userId, articleId);
    final entry = _memory[key];
    if (entry == null) return null;
    if (!entry.expiresAt.isAfter(DateTime.now().toUtc())) {
      _memory.remove(key);
      unawaited(_removePersisted(key));
      return null;
    }
    return entry.article;
  }

  static Future<Article?> get(String userId, String articleId) async {
    final inMemory = peek(userId, articleId);
    if (inMemory != null) return inMemory;

    final preferences = await _prefs();
    final key = _key(userId, articleId);
    final encoded = preferences.getString(key);
    if (encoded == null) return null;
    final entry = _decode(encoded);
    if (entry == null || !entry.expiresAt.isAfter(DateTime.now().toUtc())) {
      await preferences.remove(key);
      return null;
    }
    _memory[key] = entry;
    return entry.article;
  }

  static Future<void> put(
    String userId,
    Map<String, dynamic> articleJson,
  ) async {
    final rawExpiry = articleJson['access_expires_at'] as String?;
    final articleId = articleJson['id'] as String?;
    final serverExpiry =
        rawExpiry == null ? null : DateTime.tryParse(rawExpiry)?.toUtc();
    final now = DateTime.now().toUtc();
    final maximumExpiry = now.add(_maximumRetention);
    final expiresAt =
        serverExpiry != null && serverExpiry.isAfter(maximumExpiry)
            ? maximumExpiry
            : serverExpiry;
    if (articleId == null || expiresAt == null || !expiresAt.isAfter(now)) {
      return;
    }

    final key = _key(userId, articleId);
    final payload = Map<String, dynamic>.from(articleJson);
    final entry = _CachedArticle(
      article: Article.fromJson(payload),
      articleJson: payload,
      expiresAt: expiresAt,
    );
    _memory[key] = entry;
    final preferences = await _prefs();
    await preferences.setString(key, entry.encode());
    await _prune(preferences);
  }

  /// Preloads one user's small, bounded grant set while their session starts,
  /// making subsequent reopens synchronous.
  static Future<void> warm(String userId) async {
    final preferences = await _prefs();
    final prefix = '$_keyPrefix$userId:';
    final now = DateTime.now().toUtc();
    final expired = <String>[];
    for (final key
        in preferences.getKeys().where((key) => key.startsWith(_keyPrefix))) {
      final encoded = preferences.getString(key);
      final entry = encoded == null ? null : _decode(encoded);
      if (entry == null || !entry.expiresAt.isAfter(now)) {
        expired.add(key);
      } else if (key.startsWith(prefix)) {
        _memory[key] = entry;
      }
    }
    for (final key in expired) {
      _memory.remove(key);
      await preferences.remove(key);
    }
  }

  static void clearMemory() => _memory.clear();

  static Future<SharedPreferences> _prefs() async =>
      _preferences ??= await SharedPreferences.getInstance();

  static _CachedArticle? _decode(String encoded) {
    try {
      final value = jsonDecode(encoded) as Map<String, dynamic>;
      final articleJson = value['article'] as Map<String, dynamic>;
      final expiresAt = DateTime.parse(value['expires_at'] as String).toUtc();
      return _CachedArticle(
        article: Article.fromJson(articleJson),
        articleJson: articleJson,
        expiresAt: expiresAt,
      );
    } catch (_) {
      return null;
    }
  }

  static Future<void> _removePersisted(String key) async {
    final preferences = await _prefs();
    await preferences.remove(key);
  }

  static Future<void> _prune(
    SharedPreferences preferences,
  ) async {
    final now = DateTime.now().toUtc();
    for (final key
        in preferences.getKeys().where((key) => key.startsWith(_keyPrefix))) {
      final encoded = preferences.getString(key);
      final entry = encoded == null ? null : _decode(encoded);
      if (entry == null || !entry.expiresAt.isAfter(now)) {
        _memory.remove(key);
        await preferences.remove(key);
      }
    }
  }

  // Test-only reset keeps SharedPreferences mock instances isolated.
  static void resetForTesting() {
    _memory.clear();
    _preferences = null;
  }
}

class _CachedArticle {
  final Article article;
  final Map<String, dynamic> articleJson;
  final DateTime expiresAt;

  const _CachedArticle({
    required this.article,
    required this.articleJson,
    required this.expiresAt,
  });

  String encode() => jsonEncode({
        'expires_at': expiresAt.toIso8601String(),
        'article': articleJson,
      });
}
