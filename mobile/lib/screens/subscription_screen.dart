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
  String? _error;
  int? _subscribingId;

  @override
  void initState() {
    super.initState();
    _loadTiers();
  }

  Future<void> _loadTiers() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });
    try {
      final data = await ApiService.getSubscriptionTiers();
      if (mounted) {
        setState(() {
          _tiers = data.map((t) => SubscriptionTier.fromJson(t as Map<String, dynamic>)).toList();
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _isLoading = false;
        });
      }
    }
  }

  Future<void> _subscribe(SubscriptionTier tier) async {
    setState(() => _subscribingId = tier.id);
    try {
      final data = await ApiService.createCheckout(tier.id);
      final url = data['checkout_url'] as String?;
      if (url != null) {
        final uri = Uri.parse(url);
        if (await canLaunchUrl(uri)) {
          await launchUrl(uri, mode: LaunchMode.externalApplication);
        }
      }
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(e.message),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _subscribingId = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Subscription Plans'),
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? _ErrorState(error: _error!, onRetry: _loadTiers)
              : RefreshIndicator(
                  onRefresh: _loadTiers,
                  child: ListView(
                    padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
                    children: [
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 16),
                        child: Text(
                          'Choose a plan that works for you',
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                          textAlign: TextAlign.center,
                        ),
                      ),
                      ..._tiers.map((tier) => _TierCard(
                            tier: tier,
                            isSubscribing: _subscribingId == tier.id,
                            onSubscribe: () => _subscribe(tier),
                          )),
                    ],
                  ),
                ),
    );
  }
}

class _TierCard extends StatelessWidget {
  final SubscriptionTier tier;
  final bool isSubscribing;
  final VoidCallback onSubscribe;

  const _TierCard({
    required this.tier,
    required this.isSubscribing,
    required this.onSubscribe,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isPremium = tier.name == 'premium';
    final isFree = tier.priceMonthly == 0;

    final borderColor = tier.isCurrent
        ? theme.colorScheme.tertiary
        : isPremium
            ? theme.colorScheme.primary
            : Colors.transparent;

    return Card(
      elevation: isPremium ? 4 : 1,
      margin: const EdgeInsets.only(bottom: 16),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(
          color: borderColor,
          width: tier.isCurrent || isPremium ? 2 : 0,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Badge row
            if (tier.isCurrent || isPremium)
              Padding(
                padding: const EdgeInsets.only(bottom: 12),
                child: _Badge(
                  label: tier.isCurrent ? 'YOUR PLAN' : 'MOST POPULAR',
                  color: tier.isCurrent
                      ? theme.colorScheme.tertiary
                      : theme.colorScheme.primary,
                  onColor: tier.isCurrent
                      ? theme.colorScheme.onTertiary
                      : theme.colorScheme.onPrimary,
                ),
              ),

            // Name
            Text(
              tier.name[0].toUpperCase() + tier.name.substring(1),
              style: theme.textTheme.headlineSmall?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),

            // Price
            Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Text(
                  isFree ? 'Free' : '\$${tier.priceMonthly.toStringAsFixed(2)}',
                  style: theme.textTheme.headlineMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: theme.colorScheme.primary,
                  ),
                ),
                if (!isFree) ...[
                  const SizedBox(width: 4),
                  Padding(
                    padding: const EdgeInsets.only(bottom: 4),
                    child: Text(
                      '/month',
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                ],
              ],
            ),
            const SizedBox(height: 16),

            // Features
            _FeatureRow(
              icon: Icons.article_outlined,
              label: tier.maxArticlesPerDay >= 999
                  ? 'Unlimited articles/day'
                  : '${tier.maxArticlesPerDay} articles/day',
            ),
            if (tier.hasPremiumAccess)
              const _FeatureRow(
                icon: Icons.star_rounded,
                label: 'Premium content access',
              ),
            const SizedBox(height: 20),

            // CTA button
            SizedBox(
              width: double.infinity,
              child: _ActionButton(
                tier: tier,
                isSubscribing: isSubscribing,
                onSubscribe: onSubscribe,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Badge extends StatelessWidget {
  final String label;
  final Color color;
  final Color onColor;

  const _Badge({
    required this.label,
    required this.color,
    required this.onColor,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(20),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
              color: onColor,
              fontWeight: FontWeight.bold,
              letterSpacing: 0.5,
            ),
      ),
    );
  }
}

class _FeatureRow extends StatelessWidget {
  final IconData icon;
  final String label;

  const _FeatureRow({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          Icon(icon, size: 18, color: theme.colorScheme.primary),
          const SizedBox(width: 10),
          Text(label, style: theme.textTheme.bodyMedium),
        ],
      ),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final SubscriptionTier tier;
  final bool isSubscribing;
  final VoidCallback onSubscribe;

  const _ActionButton({
    required this.tier,
    required this.isSubscribing,
    required this.onSubscribe,
  });

  @override
  Widget build(BuildContext context) {
    if (tier.isCurrent) {
      return const FilledButton.tonal(
        onPressed: null,
        child: Text('Current Plan'),
      );
    }
    if (tier.priceMonthly == 0) {
      return const FilledButton.tonal(
        onPressed: null,
        child: Text('Free'),
      );
    }
    return FilledButton(
      onPressed: isSubscribing ? null : onSubscribe,
      child: isSubscribing
          ? const SizedBox(
              height: 18,
              width: 18,
              child: CircularProgressIndicator(strokeWidth: 2),
            )
          : const Text('Subscribe'),
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String error;
  final VoidCallback onRetry;

  const _ErrorState({required this.error, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline_rounded,
                size: 56, color: theme.colorScheme.onSurfaceVariant),
            const SizedBox(height: 16),
            Text('Could not load plans', style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              error,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            FilledButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh_rounded),
              label: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }
}
