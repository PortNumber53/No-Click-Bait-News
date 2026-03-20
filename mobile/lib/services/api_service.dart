import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class ApiService {
  static const String baseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://10.0.2.2:8000/api/v1',
  );
  static const _storage = FlutterSecureStorage();

  static Future<String?> _getToken() async {
    return await _storage.read(key: 'access_token');
  }

  static Future<void> saveToken(String token) async {
    await _storage.write(key: 'access_token', value: token);
  }

  static Future<void> clearToken() async {
    await _storage.delete(key: 'access_token');
  }

  static Future<Map<String, String>> _headers({bool auth = false}) async {
    final headers = {'Content-Type': 'application/json'};
    if (auth) {
      final token = await _getToken();
      if (token != null) {
        headers['Authorization'] = 'Bearer $token';
      }
    }
    return headers;
  }

  // Auth
  static Future<Map<String, dynamic>> register(
      String email, String password, String name) async {
    final response = await http.post(
      Uri.parse('$baseUrl/auth/register'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'email': email, 'password': password, 'name': name}),
    );
    if (response.statusCode == 201) {
      return jsonDecode(response.body);
    }
    throw ApiException(response.statusCode, jsonDecode(response.body)['detail']);
  }

  static Future<Map<String, dynamic>> login(
      String email, String password) async {
    final response = await http.post(
      Uri.parse('$baseUrl/auth/login'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'email': email, 'password': password}),
    );
    if (response.statusCode == 200) {
      return jsonDecode(response.body);
    }
    throw ApiException(response.statusCode, jsonDecode(response.body)['detail']);
  }

  // Articles
  static Future<Map<String, dynamic>> getFeed({
    int page = 1,
    int pageSize = 20,
    String? category,
  }) async {
    final params = {
      'page': page.toString(),
      'page_size': pageSize.toString(),
      if (category != null) 'category': category,
    };
    final uri = Uri.parse('$baseUrl/articles/feed')
        .replace(queryParameters: params);
    final response = await http.get(uri, headers: await _headers(auth: true));
    if (response.statusCode == 200) {
      return jsonDecode(response.body);
    }
    throw ApiException(response.statusCode, 'Failed to fetch articles');
  }

  static Future<Map<String, dynamic>> getArticle(String id) async {
    final response = await http.get(
      Uri.parse('$baseUrl/articles/$id'),
      headers: await _headers(auth: true),
    );
    if (response.statusCode == 200) {
      return jsonDecode(response.body);
    }
    throw ApiException(response.statusCode, 'Failed to fetch article');
  }

  // Subscriptions
  static Future<List<dynamic>> getSubscriptionTiers() async {
    final response = await http.get(
      Uri.parse('$baseUrl/subscriptions/tiers'),
      headers: await _headers(),
    );
    if (response.statusCode == 200) {
      return jsonDecode(response.body);
    }
    throw ApiException(response.statusCode, 'Failed to fetch tiers');
  }

  static Future<Map<String, dynamic>> createCheckout(int tierId) async {
    final response = await http.post(
      Uri.parse('$baseUrl/subscriptions/checkout'),
      headers: await _headers(auth: true),
      body: jsonEncode({'tier_id': tierId}),
    );
    if (response.statusCode == 200) {
      return jsonDecode(response.body);
    }
    throw ApiException(response.statusCode, 'Failed to create checkout');
  }
}

class ApiException implements Exception {
  final int statusCode;
  final String message;
  const ApiException(this.statusCode, this.message);

  @override
  String toString() => message;
}
