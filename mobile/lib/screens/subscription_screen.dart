import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/subscription_tier.dart';
import '../providers/auth_provider.dart';
import '../services/api_service.dart';

class SubscriptionScreen extends StatefulWidget {
  const SubscriptionScreen({super.key});

  @override
  State<SubscriptionScreen> createState() => _SubscriptionScreenState();
}

class _SubscriptionScreenState extends State<SubscriptionScreen>
    with WidgetsBindingObserver {
  List<SubscriptionTier> _tiers = [];
  bool _isLoading = true;
  String? _error;
  int? _subscribingId;
  bool _isOpeningPortal = false;
  bool _waitingForCheckout = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _loadTiers();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && _waitingForCheckout) {
      _loadTiers(showLoading: false);
    }
  }

  Future<void> _loadTiers({bool showLoading = true}) async {
    if (showLoading) {
      setState(() {
        _isLoading = true;
        _error = null;
      });
    }
    try {
      final data = await ApiService.getSubscriptionTiers();
      if (mounted) {
        final tiers = data
            .map((t) => SubscriptionTier.fromJson(t as Map<String, dynamic>))
            .toList();
        final becameUnlimited =
            tiers.any((tier) => tier.isCurrent && tier.unlimitedReading);
        setState(() {
          _tiers = tiers;
          _isLoading = false;
          _error = null;
          if (becameUnlimited) {
            _waitingForCheckout = false;
          }
        });
        if (becameUnlimited) {
          await context.read<AuthProvider>().refreshUser();
        }
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
      if (url == null) {
        throw const ApiException(502, 'Stripe checkout is unavailable');
      }
      final launched = await launchUrl(
        Uri.parse(url),
        mode: LaunchMode.externalApplication,
      );
      if (!launched) {
        throw const ApiException(502, 'Could not open Stripe checkout');
      }
      if (mounted) {
        setState(() => _waitingForCheckout = true);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'Finish secure checkout in Stripe, then return here.',
            ),
          ),
        );
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
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Could not open Stripe checkout'),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _subscribingId = null);
    }
  }

  Future<void> _manageBilling() async {
    if (_isOpeningPortal) return;
    setState(() => _isOpeningPortal = true);
    try {
      final data = await ApiService.createBillingPortal();
      final url = data['portal_url'] as String?;
      if (url == null ||
          !await launchUrl(
            Uri.parse(url),
            mode: LaunchMode.externalApplication,
          )) {
        throw const ApiException(502, 'Could not open Stripe billing');
      }
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.message)),
        );
      }
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Could not open Stripe billing')),
        );
      }
    } finally {
      if (mounted) setState(() => _isOpeningPortal = false);
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
                      Container(
                        margin: const EdgeInsets.only(bottom: 18),
                        padding: const EdgeInsets.all(18),
                        decoration: BoxDecoration(
                          gradient: const LinearGradient(
                            colors: [Color(0xFF17324D), Color(0xFF087F74)],
                          ),
                          borderRadius: BorderRadius.circular(20),
                        ),
                        child: Row(
                          children: [
                            const Icon(
                              Icons.lock_rounded,
                              color: Colors.white,
                              size: 30,
                            ),
                            const SizedBox(width: 14),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    'Simple, secure billing',
                                    style:
                                        theme.textTheme.titleMedium?.copyWith(
                                      color: Colors.white,
                                    ),
                                  ),
                                  const SizedBox(height: 3),
                                  Text(
                                    'Payments and invoices are handled by Stripe.',
                                    style: theme.textTheme.bodySmall?.copyWith(
                                      color: Colors.white70,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                      if (_waitingForCheckout)
                        Card(
                          color: theme.colorScheme.tertiaryContainer,
                          margin: const EdgeInsets.only(bottom: 16),
                          child: Padding(
                            padding: const EdgeInsets.all(16),
                            child: Row(
                              children: [
                                const Icon(Icons.hourglass_top_rounded),
                                const SizedBox(width: 12),
                                const Expanded(
                                  child: Text(
                                    'Waiting for Stripe to confirm your plan.',
                                  ),
                                ),
                                TextButton(
                                  onPressed: () =>
                                      _loadTiers(showLoading: false),
                                  child: const Text('Refresh'),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ..._tiers.map((tier) => _TierCard(
                            tier: tier,
                            isSubscribing: _subscribingId == tier.id,
                            isOpeningPortal: _isOpeningPortal,
                            onSubscribe: () => _subscribe(tier),
                            onManageBilling: _manageBilling,
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
  final bool isOpeningPortal;
  final VoidCallback onSubscribe;
  final VoidCallback onManageBilling;

  const _TierCard({
    required this.tier,
    required this.isSubscribing,
    required this.isOpeningPortal,
    required this.onSubscribe,
    required this.onManageBilling,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isPremium = tier.unlimitedReading;
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
              isPremium ? 'Unlimited' : 'Free',
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
              label: tier.unlimitedReading
                  ? 'Unlimited news reading'
                  : '1 article per category, every day',
            ),
            if (tier.unlimitedReading) ...[
              const _FeatureRow(
                icon: Icons.star_rounded,
                label: 'Every category and premium story',
              ),
              const _FeatureRow(
                icon: Icons.link_rounded,
                label: 'Unlimited submitted news links',
              ),
            ] else
              const _FeatureRow(
                icon: Icons.category_rounded,
                label: 'A fresh choice in each category',
              ),
            const SizedBox(height: 20),

            // CTA button
            SizedBox(
              width: double.infinity,
              child: _ActionButton(
                tier: tier,
                isSubscribing: isSubscribing,
                isOpeningPortal: isOpeningPortal,
                onSubscribe: onSubscribe,
                onManageBilling: onManageBilling,
              ),
            ),
            if (!isFree) ...[
              const SizedBox(height: 10),
              Center(
                child: Text(
                  'Renews monthly. Cancel anytime in Stripe.',
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            ],
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
  final bool isOpeningPortal;
  final VoidCallback onSubscribe;
  final VoidCallback onManageBilling;

  const _ActionButton({
    required this.tier,
    required this.isSubscribing,
    required this.isOpeningPortal,
    required this.onSubscribe,
    required this.onManageBilling,
  });

  @override
  Widget build(BuildContext context) {
    if (tier.isCurrent) {
      if (tier.unlimitedReading) {
        return OutlinedButton.icon(
          onPressed: isOpeningPortal ? null : onManageBilling,
          icon: isOpeningPortal
              ? const SizedBox(
                  height: 18,
                  width: 18,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.open_in_new_rounded),
          label: const Text('Manage billing with Stripe'),
        );
      }
      return const FilledButton.tonal(
        onPressed: null,
        child: Text('Current plan'),
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
          : const Text('Continue with Stripe'),
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
