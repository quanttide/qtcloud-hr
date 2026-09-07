import 'package:flutter/material.dart';

import 'platform/session_storage.dart';
import 'screens/compensation_screen.dart';
import 'screens/overview_screen.dart';
import 'screens/performance_screen.dart';
import 'screens/plan_screen.dart';
import 'screens/recruitment_screen.dart';
import 'screens/relation_screen.dart';
import 'screens/training_screen.dart';
import 'services/auth_client.dart';
import 'services/recruitment_api_client.dart';

const _authBaseUrl = String.fromEnvironment(
  'QTCLOUD_AUTH_BASE_URL',
  defaultValue: 'https://api.quanttide.com/qtcloud-auth',
);
const _apiBaseUrl = String.fromEnvironment('QTCLOUD_HUMAN_API_BASE_URL');
const _operator = String.fromEnvironment(
  'QTCLOUD_HUMAN_OPERATOR',
  defaultValue: 'studio-user',
);
const _sessionTokenKey = 'qtcloud-human.access-token';
const _sessionRefreshTokenKey = 'qtcloud-human.refresh-token';

void main() => runApp(const WorkbenchApp());

class WorkbenchApp extends StatelessWidget {
  const WorkbenchApp({super.key, this.initialAccessToken});

  final String? initialAccessToken;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '量潮人事云工作台',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF1E40AF)),
        useMaterial3: true,
        fontFamily: 'PingFang SC',
      ),
      home: AuthGate(initialAccessToken: initialAccessToken),
    );
  }
}

class AuthGate extends StatefulWidget {
  const AuthGate({super.key, this.initialAccessToken});

  final String? initialAccessToken;

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  String? _accessToken;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _restoreSession();
  }

  Future<void> _restoreSession() async {
    final initialAccessToken = widget.initialAccessToken;
    if (initialAccessToken != null && initialAccessToken.isNotEmpty) {
      if (mounted) {
        setState(() {
          _accessToken = initialAccessToken;
          _loading = false;
        });
      }
      return;
    }

    final refreshToken = readSessionValue(_sessionRefreshTokenKey);
    if (refreshToken == null || refreshToken.trim().isEmpty) {
      _clearSession();
      return;
    }

    try {
      final tokens = await AuthClient(
        baseUrl: _authBaseUrl,
      ).refresh(refreshToken: refreshToken);
      if (!mounted) {
        return;
      }
      _persistSession(tokens);
      setState(() {
        _accessToken = tokens.accessToken;
        _loading = false;
      });
    } on AuthClientException {
      _clearSession();
    } catch (_) {
      _clearSession();
    }
  }

  void _onAuthenticated(AuthTokens tokens) {
    _persistSession(tokens);
    setState(() => _accessToken = tokens.accessToken);
  }

  void _persistSession(AuthTokens tokens) {
    writeSessionValue(_sessionTokenKey, tokens.accessToken);
    writeSessionValue(_sessionRefreshTokenKey, tokens.refreshToken);
  }

  void _clearSession() {
    removeSessionValue(_sessionTokenKey);
    removeSessionValue(_sessionRefreshTokenKey);
    if (mounted) {
      setState(() {
        _accessToken = null;
        _loading = false;
      });
    }
  }

  void _logout() {
    _clearSession();
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    final token = _accessToken;
    if (token == null || token.isEmpty) {
      return LoginScreen(onAuthenticated: _onAuthenticated);
    }
    return WorkbenchShell(accessToken: token, onLogout: _logout);
  }
}

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key, required this.onAuthenticated});

  final ValueChanged<AuthTokens> onAuthenticated;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _usernameController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _loading = false;
  String? _error;

  @override
  void dispose() {
    _usernameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _login() async {
    final username = _usernameController.text.trim();
    final password = _passwordController.text;
    if (username.isEmpty || password.isEmpty) {
      setState(() => _error = '请输入账号和密码');
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final tokens = await AuthClient(
        baseUrl: _authBaseUrl,
      ).login(username: username, password: password);
      if (mounted) {
        widget.onAuthenticated(tokens);
      }
    } on AuthClientException catch (error) {
      if (mounted) {
        setState(() => _error = error.message);
      }
    } catch (_) {
      if (mounted) {
        setState(() => _error = '认证服务暂不可用，请稍后重试');
      }
    } finally {
      if (mounted) {
        setState(() => _loading = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: Card(
            margin: const EdgeInsets.all(24),
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: AutofillGroup(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      '量潮人事云',
                      style: Theme.of(context).textTheme.headlineSmall,
                    ),
                    const SizedBox(height: 24),
                    TextField(
                      controller: _usernameController,
                      autofillHints: const [AutofillHints.username],
                      decoration: const InputDecoration(
                        labelText: '账号 / 邮箱 / 手机号',
                        border: OutlineInputBorder(),
                      ),
                      onSubmitted: (_) => _login(),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: _passwordController,
                      obscureText: true,
                      autofillHints: const [AutofillHints.password],
                      decoration: const InputDecoration(
                        labelText: '密码',
                        border: OutlineInputBorder(),
                      ),
                      onSubmitted: (_) => _login(),
                    ),
                    if (_error != null) ...[
                      const SizedBox(height: 12),
                      Text(
                        _error!,
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.error,
                        ),
                      ),
                    ],
                    const SizedBox(height: 20),
                    FilledButton.icon(
                      onPressed: _loading ? null : _login,
                      icon: _loading
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.login),
                      label: Text(_loading ? '登录中' : '登录'),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 工作台骨架：左侧导航栏 + 模块页面。
///
/// 各模块独立文件、独立 Scaffold，通过侧边导航切换。
class WorkbenchShell extends StatefulWidget {
  const WorkbenchShell({super.key, this.accessToken = '', this.onLogout});

  final String accessToken;
  final VoidCallback? onLogout;

  @override
  State<WorkbenchShell> createState() => _WorkbenchShellState();
}

class _WorkbenchShellState extends State<WorkbenchShell> {
  int _index = 0;
  late final List<Widget> _pages;

  @override
  void initState() {
    super.initState();
    _pages = [
      const OverviewScreen(),
      const PlanScreen(),
      RecruitmentPage(
        apiClient: _apiBaseUrl.isEmpty
            ? null
            : RecruitmentApiClient(
                baseUrl: _apiBaseUrl,
                accessToken: widget.accessToken,
                operator: _operator,
              ),
      ),
      const TrainingScreen(),
      const PerformanceScreen(),
      const CompensationScreen(),
      const RelationScreen(),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            selectedIndex: _index,
            onDestinationSelected: (i) => setState(() => _index = i),
            labelType: NavigationRailLabelType.all,
            leading: Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Column(
                children: [
                  Icon(Icons.workspaces, size: 28, color: colorScheme.primary),
                  const SizedBox(height: 4),
                  Text(
                    '量潮人事',
                    style: TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.w600,
                      color: colorScheme.primary,
                    ),
                  ),
                ],
              ),
            ),
            destinations: const [
              NavigationRailDestination(
                icon: Icon(Icons.dashboard_outlined),
                selectedIcon: Icon(Icons.dashboard),
                label: Text('总览'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.event_note_outlined),
                selectedIcon: Icon(Icons.event_note),
                label: Text('计划'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.mail_outline),
                selectedIcon: Icon(Icons.mail),
                label: Text('招聘'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.school_outlined),
                selectedIcon: Icon(Icons.school),
                label: Text('培训'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.assessment_outlined),
                selectedIcon: Icon(Icons.assessment),
                label: Text('绩效'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.payments_outlined),
                selectedIcon: Icon(Icons.payments),
                label: Text('薪酬'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.groups_outlined),
                selectedIcon: Icon(Icons.groups),
                label: Text('关系'),
              ),
            ],
            trailing: widget.onLogout == null
                ? null
                : IconButton(
                    tooltip: '退出登录',
                    onPressed: widget.onLogout,
                    icon: const Icon(Icons.logout),
                  ),
          ),
          const VerticalDivider(width: 1),
          Expanded(child: _pages[_index]),
        ],
      ),
    );
  }
}
