import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/subscription_tier.dart';
import '../services/api_service.dart';

class SubscriptionScreen extends StatefulWidget {
  const SubscriptionScreen({super.key});

  @override
  State<SubscriptionScreen> createState() => _SubscriptionScreenState();
}

class _SubscriptionScreenState extends State<SubscriptionScreen> {
  List<SubscriptionTier> _tiers = [];
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadTiers();
  }

  Future<void> _loadTiers() async {
    try {
      final data = await ApiService.getSubscriptionTiers();
      setState(() {
        _tiers = data.map((t) => SubscriptionTier.fromJson(t)).toList();
        _isLoading = false;
      });
    } catch (e) {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _subscribe(SubscriptionTier tier) async {
    try {
      final data = await ApiService.createCheckout(tier.id);
      final url = data['checkout_url'];
      if (url != null) {
        final uri = Uri.parse(url);
        if (await canLaunchUrl(uri)) {
          await launchUrl(uri, mode: LaunchMode.externalApplication);
        }
      }
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.message)),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Subscription Plans')),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : ListView.builder(
              padding: const EdgeInsets.all(16),
              itemCount: _tiers.length,
              itemBuilder: (context, index) {
                final tier = _tiers[index];
                final isPremium = tier.name == 'premium';
                return Card(
                  elevation: isPremium ? 4 : 1,
                  margin: const EdgeInsets.only(bottom: 16),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                    side: isPremium
                        ? BorderSide(color: theme.colorScheme.primary, width: 2)
                        : BorderSide.none,
                  ),
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (isPremium)
                          Container(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 12, vertical: 4),
                            decoration: BoxDecoration(
                              color: theme.colorScheme.primary,
                              borderRadius: BorderRadius.circular(20),
                            ),
                            child: Text(
                              'MOST POPULAR',
                              style: theme.textTheme.labelSmall?.copyWith(
                                color: theme.colorScheme.onPrimary,
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                          ),
                        if (isPremium) const SizedBox(height: 12),
                        Text(
                          tier.name[0].toUpperCase() + tier.name.substring(1),
                          style: theme.textTheme.headlineSmall?.copyWith(
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        const SizedBox(height: 8),
                        RichText(
                          text: TextSpan(
                            style: theme.textTheme.bodyMedium,
                            children: [
                              TextSpan(
                                text: '\$${tier.priceMonthly.toStringAsFixed(2)}',
                                style: theme.textTheme.headlineMedium?.copyWith(
                                  fontWeight: FontWeight.bold,
                                  color: theme.colorScheme.primary,
                                ),
                              ),
                              const TextSpan(text: ' /month'),
                            ],
                          ),
                        ),
                        const SizedBox(height: 16),
                        _featureRow(
                          Icons.article_outlined,
                          '${tier.maxArticlesPerDay} articles/day',
                        ),
                        if (tier.hasPremiumAccess)
                          _featureRow(
                            Icons.star_outline,
                            'Premium content access',
                          ),
                        const SizedBox(height: 20),
                        SizedBox(
                          width: double.infinity,
                          child: tier.priceMonthly > 0
                              ? FilledButton(
                                  onPressed: () => _subscribe(tier),
                                  child: const Text('Subscribe'),
                                )
                              : FilledButton.tonal(
                                  onPressed: null,
                                  child: const Text('Current Plan'),
                                ),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
    );
  }

  Widget _featureRow(IconData icon, String text) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          Icon(icon, size: 20, color: Theme.of(context).colorScheme.primary),
          const SizedBox(width: 8),
          Text(text),
        ],
      ),
    );
  }
}
