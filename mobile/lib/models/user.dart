class User {
  final String id;
  final String email;
  final String name;
  final String? subscriptionTier;

  const User({
    required this.id,
    required this.email,
    required this.name,
    this.subscriptionTier,
  });

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'],
      email: json['email'],
      name: json['name'],
      subscriptionTier: json['subscription_tier'],
    );
  }
}
