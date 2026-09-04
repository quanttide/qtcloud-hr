import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../services/recruitment_api_client.dart';

const _apiBaseUrl = String.fromEnvironment('QTCLOUD_HUMAN_API_BASE_URL');
const _operator = String.fromEnvironment(
  'QTCLOUD_HUMAN_OPERATOR',
  defaultValue: 'studio-user',
);

/// 招聘筛选模块：投递邮件初筛，后端可用时调用 provider API。
///
/// 无 API 配置时保留 assets/mock/emails.json 作为开发 fallback。
enum DetectResult { only, has_ }

enum EmailStatus { pending, passed, rejected }

class Email {
  final int id;
  final String candidateId;
  final String from;
  final String name;
  final String subject;
  final String body;
  final bool hasCover;
  final bool hasBody;
  final bool hasResume;
  final String extra;
  final String position;
  final String stage;
  final String lastAction;
  final List<String> tags;
  late final DetectResult detected;
  EmailStatus status;

  Email({
    required this.id,
    required this.candidateId,
    required this.from,
    required this.name,
    required this.subject,
    required this.body,
    required this.hasCover,
    required this.hasBody,
    required this.hasResume,
    required this.extra,
    required this.position,
    required this.stage,
    required this.lastAction,
    required this.tags,
    this.status = EmailStatus.pending,
  }) {
    detected = (hasCover || (hasBody && body.length > 20))
        ? DetectResult.has_
        : DetectResult.only;
    if (detected == DetectResult.only) status = EmailStatus.rejected;
  }

  factory Email.fromJson(Map<String, dynamic> json) => Email(
    id: json['id'] as int,
    candidateId: 'cand_${json['id']}',
    from: json['from'] as String,
    name: json['name'] as String,
    subject: json['subject'] as String,
    body: (json['body'] as String?) ?? '',
    hasCover: (json['hasCover'] as bool?) ?? false,
    hasBody: (json['hasBody'] as bool?) ?? false,
    hasResume: (json['hasResume'] as bool?) ?? false,
    extra: (json['extra'] as String?) ?? '',
    position: _positionFromSubject((json['subject'] as String?) ?? ''),
    stage: 'mock',
    lastAction: '',
    tags: (json['tags'] as List<dynamic>? ?? []).cast<String>(),
  );

  factory Email.fromCandidate(RecruitmentCandidate candidate, int index) {
    return Email(
      id: index + 1,
      candidateId: candidate.id,
      from: candidate.email,
      name: candidate.name,
      subject: candidate.position.isEmpty ? '候选人投递' : candidate.position,
      body: '来自 provider API 的候选人记录。当前阶段：${candidate.stage}',
      hasCover: candidate.hasCoverLetter,
      hasBody: candidate.hasCoverLetter,
      hasResume: candidate.hasResume,
      extra: candidate.lastAction.isEmpty ? '' : '最近动作：${candidate.lastAction}',
      position: candidate.position,
      stage: candidate.stage,
      lastAction: candidate.lastAction,
      tags: [
        candidate.stage,
        if (candidate.lastAction.isNotEmpty) candidate.lastAction,
      ],
    );
  }
}

class RecruitmentPage extends StatefulWidget {
  const RecruitmentPage({super.key});

  @override
  State<RecruitmentPage> createState() => _RecruitmentPageState();
}

class _RecruitmentPageState extends State<RecruitmentPage> {
  final RecruitmentApiClient? _apiClient = _apiBaseUrl.isEmpty
      ? null
      : RecruitmentApiClient(baseUrl: _apiBaseUrl, operator: _operator);
  final TextEditingController _interviewPositionController =
      TextEditingController();
  final TextEditingController _interviewTimeController = TextEditingController(
    text: _defaultInterviewTime(),
  );

  Future<List<Email>>? _future;
  late List<Email> _emails;
  int? _selectedId;
  bool _dryRun = true;
  bool _reportLoading = false;
  String? _reportMarkdown;
  String? _reportStatus;
  String? _lastActionMessage;
  String? _lastActionError;
  RecruitmentAction? _runningAction;

  @override
  void initState() {
    super.initState();
    _future = _loadEmails();
  }

  @override
  void dispose() {
    _interviewPositionController.dispose();
    _interviewTimeController.dispose();
    super.dispose();
  }

  Future<List<Email>> _loadEmails() async {
    if (_apiClient != null) {
      try {
        final candidates = await _apiClient.listCandidates();
        final emails = candidates
            .asMap()
            .entries
            .map((entry) => Email.fromCandidate(entry.value, entry.key))
            .toList();
        _selectedId = emails.isNotEmpty ? emails.first.id : null;
        _syncInterviewInputs(emails.isNotEmpty ? emails.first : null);
        return emails;
      } on RecruitmentApiException {
        rethrow;
      }
    }
    final raw = await rootBundle.loadString('assets/mock/emails.json');
    final list = jsonDecode(raw) as List<dynamic>;
    final emails = list
        .cast<Map<String, dynamic>>()
        .map(Email.fromJson)
        .toList();
    _selectedId = emails.isNotEmpty ? emails.first.id : null;
    _syncInterviewInputs(emails.isNotEmpty ? emails.first : null);
    return emails;
  }

  int get _total => _emails.length;
  int get _onlyCount =>
      _emails.where((e) => e.detected == DetectResult.only).length;
  int get _hasCount =>
      _emails.where((e) => e.detected == DetectResult.has_).length;
  int get _processed =>
      _emails.where((e) => e.status != EmailStatus.pending).length;

  Email? get _selected => _emails.where((e) => e.id == _selectedId).firstOrNull;

  void _mark(int id, EmailStatus status) {
    setState(() {
      final e = _emails.where((x) => x.id == id).firstOrNull;
      if (e != null) e.status = status;
    });
  }

  void _selectEmail(Email email) {
    setState(() {
      _selectedId = email.id;
      _syncInterviewInputs(email);
    });
  }

  void _syncInterviewInputs(Email? email) {
    _interviewPositionController.text = email?.position ?? '';
    if (_interviewTimeController.text.trim().isEmpty) {
      _interviewTimeController.text = _defaultInterviewTime();
    }
  }

  Future<void> _createReport() async {
    setState(() {
      _reportLoading = true;
      _lastActionError = null;
      _lastActionMessage = null;
    });
    try {
      final report = _apiClient == null
          ? RecruitmentReport(
              reportId: 'mock_report',
              status: _dryRun ? 'dry_run' : 'created',
              markdown: _mockReportMarkdown(),
            )
          : await _apiClient.createReport(days: 30, dryRun: _dryRun);
      setState(() {
        _reportMarkdown = report.markdown;
        _reportStatus = report.status;
        _lastActionMessage = report.status == 'dry_run'
            ? '报告 dry_run 已生成，未读取真实邮箱。'
            : '招聘统计报告已生成。';
      });
    } on RecruitmentApiException catch (error) {
      setState(() => _lastActionError = error.message);
    } finally {
      if (mounted) {
        setState(() => _reportLoading = false);
      }
    }
  }

  Future<void> _runCandidateAction(
    Email email,
    RecruitmentAction action,
  ) async {
    setState(() {
      _runningAction = action;
      _lastActionError = null;
      _lastActionMessage = null;
    });
    try {
      final params = _paramsForAction(email, action);
      final result = _apiClient == null
          ? RecruitmentActionResult(
              actionId: 'mock_action',
              candidateId: email.candidateId,
              action: action.value,
              status: _dryRun
                  ? 'dry_run'
                  : action == RecruitmentAction.createInterviewNotice
                  ? 'draft'
                  : 'sent',
              message: _dryRun
                  ? '已完成 dry_run 预览，未发送邮件'
                  : _actionDoneMessage(action),
            )
          : await _apiClient.runAction(
              candidateId: email.candidateId,
              action: action,
              dryRun: _dryRun,
              params: params,
            );
      setState(() {
        _lastActionMessage = '${_actionLabel(action)}：${result.message}';
      });
    } on RecruitmentApiException catch (error) {
      setState(() => _lastActionError = error.message);
    } finally {
      if (mounted) {
        setState(() => _runningAction = null);
      }
    }
  }

  Map<String, Object?> _paramsForAction(Email email, RecruitmentAction action) {
    switch (action) {
      case RecruitmentAction.sendSurvey:
        return const <String, Object?>{};
      case RecruitmentAction.sendTrainingInvite:
        return const <String, Object?>{};
      case RecruitmentAction.sendExam:
        return const <String, Object?>{};
      case RecruitmentAction.createInterviewNotice:
        return <String, Object?>{
          'position': _interviewPositionController.text.trim().isEmpty
              ? '待确认岗位'
              : _interviewPositionController.text.trim(),
          'time': _interviewTimeController.text.trim().isEmpty
              ? _defaultInterviewTime()
              : _interviewTimeController.text.trim(),
        };
    }
  }

  Color _detectColor(DetectResult d) => d == DetectResult.only
      ? const Color(0xFF991B1B)
      : const Color(0xFF166534);

  Color _detectBg(DetectResult d) => d == DetectResult.only
      ? const Color(0xFFFEE2E2)
      : const Color(0xFFDCFCE7);

  String _detectLabel(DetectResult d) => d == DetectResult.only ? '仅简历' : '有正文';

  String _statusLabel(EmailStatus s) => s == EmailStatus.pending
      ? '待处理'
      : s == EmailStatus.passed
      ? '已通过'
      : '已拒绝';

  IconData _detectIcon(DetectResult d) => d == DetectResult.only
      ? Icons.description_outlined
      : Icons.email_outlined;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('招聘筛选网关'),
        centerTitle: false,
        actions: [
          Row(
            children: [
              const Text('dry_run'),
              Switch(
                value: _dryRun,
                onChanged: (value) => setState(() => _dryRun = value),
              ),
              Padding(
                padding: const EdgeInsets.only(right: 12),
                child: FilledButton.icon(
                  onPressed: _reportLoading ? null : _createReport,
                  icon: _reportLoading
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.summarize_outlined, size: 18),
                  label: Text(_reportLoading ? '生成中' : '生成报告'),
                ),
              ),
            ],
          ),
        ],
      ),
      body: FutureBuilder<List<Email>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(child: Text('加载失败：${_safeError(snapshot.error)}'));
          }
          _emails = snapshot.data!;
          return Column(
            children: [
              _buildStats(),
              _buildFeedbackBar(),
              Expanded(
                child: Row(
                  children: [
                    SizedBox(width: 360, child: _buildEmailList()),
                    const VerticalDivider(width: 1),
                    Expanded(child: _buildDetail()),
                  ],
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  Widget _buildFeedbackBar() {
    if (_lastActionMessage == null &&
        _lastActionError == null &&
        _reportMarkdown == null) {
      return const SizedBox.shrink();
    }
    final isError = _lastActionError != null;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      decoration: BoxDecoration(
        color: isError ? const Color(0xFFFEE2E2) : const Color(0xFFEFF6FF),
        border: Border(bottom: BorderSide(color: Colors.grey.shade300)),
      ),
      child: Text(
        isError
            ? _lastActionError!
            : _lastActionMessage ?? '报告状态：$_reportStatus',
        style: TextStyle(
          color: isError ? const Color(0xFF991B1B) : const Color(0xFF1E40AF),
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  Widget _buildStats() {
    return Container(
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: Colors.grey.shade300)),
      ),
      child: Row(
        children: [
          _statItem('总邮件', _total.toString()),
          _statItem(
            '仅简历',
            _onlyCount.toString(),
            color: const Color(0xFF991B1B),
          ),
          _statItem(
            '有正文',
            _hasCount.toString(),
            color: const Color(0xFF166534),
          ),
          _statItem('自动处理', '$_processed/$_total'),
        ],
      ),
    );
  }

  Widget _statItem(String label, String value, {Color? color}) {
    return Expanded(
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 12),
        decoration: BoxDecoration(
          border: Border(right: BorderSide(color: Colors.grey.shade300)),
        ),
        child: Column(
          children: [
            Text(
              value,
              style: TextStyle(
                fontSize: 22,
                fontWeight: FontWeight.w700,
                color: color ?? Theme.of(context).colorScheme.primary,
              ),
            ),
            const SizedBox(height: 2),
            Text(
              label,
              style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildEmailList() {
    return Column(
      children: [
        Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          decoration: BoxDecoration(
            border: Border(bottom: BorderSide(color: Colors.grey.shade300)),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                '收件箱',
                style: TextStyle(fontWeight: FontWeight.w600, fontSize: 14),
              ),
              Text(
                '${_emails.length} 封',
                style: TextStyle(fontSize: 12, color: Colors.grey.shade500),
              ),
            ],
          ),
        ),
        Expanded(
          child: ListView.builder(
            itemCount: _emails.length,
            itemBuilder: (_, i) => _buildEmailTile(_emails[i]),
          ),
        ),
      ],
    );
  }

  Widget _buildEmailTile(Email e) {
    final sel = e.id == _selectedId;
    final dc = _detectColor(e.detected);
    return InkWell(
      onTap: () => _selectEmail(e),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        decoration: BoxDecoration(
          color: sel ? const Color(0xFFEFF6FF) : null,
          border: Border(bottom: BorderSide(color: Colors.grey.shade200)),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  e.name,
                  style: const TextStyle(
                    fontWeight: FontWeight.w600,
                    fontSize: 14,
                  ),
                ),
                Icon(
                  _detectIcon(e.detected),
                  size: 18,
                  color: Colors.grey.shade400,
                ),
              ],
            ),
            const SizedBox(height: 2),
            Text(
              e.subject,
              style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
            ),
            const SizedBox(height: 6),
            Wrap(
              spacing: 6,
              runSpacing: 4,
              children: [
                _tag(_detectLabel(e.detected), _detectBg(e.detected), dc),
                ...e.tags.map(
                  (t) =>
                      _tag(t, const Color(0xFFE0E7FF), const Color(0xFF1E40AF)),
                ),
                Text(
                  _statusLabel(e.status),
                  style: TextStyle(fontSize: 12, color: Colors.grey.shade500),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _tag(String text, Color bg, Color color) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w500,
          color: color,
        ),
      ),
    );
  }

  Widget _buildDetail() {
    final e = _selected;
    if (e == null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.email_outlined, size: 48, color: Colors.grey.shade300),
            const SizedBox(height: 8),
            Text('选择一封邮件查看', style: TextStyle(color: Colors.grey.shade400)),
          ],
        ),
      );
    }

    final dc = _detectColor(e.detected);
    final dbg = _detectBg(e.detected);

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              _tag(_detectLabel(e.detected), dbg, dc),
              const SizedBox(width: 8),
              _tag(
                _apiClient == null ? 'mock fallback' : 'provider API',
                const Color(0xFFF1F5F9),
                const Color(0xFF334155),
              ),
              const Spacer(),
              Text(
                e.status == EmailStatus.passed
                    ? '已通过'
                    : e.status == EmailStatus.rejected
                    ? '已拒绝'
                    : '',
                style: TextStyle(
                  fontSize: 13,
                  color: e.status == EmailStatus.passed
                      ? const Color(0xFF166534)
                      : const Color(0xFF991B1B),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Text(
            e.name,
            style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 4),
          Text(
            '<${e.from}>',
            style: TextStyle(fontSize: 13, color: Colors.grey.shade600),
          ),
          const SizedBox(height: 2),
          Text(
            e.subject,
            style: TextStyle(fontSize: 13, color: Colors.grey.shade600),
          ),
          if (e.position.isNotEmpty) ...[
            const SizedBox(height: 2),
            Text(
              '岗位：${e.position}',
              style: TextStyle(fontSize: 13, color: Colors.grey.shade600),
            ),
          ],
          const SizedBox(height: 16),
          _bodyCard(e),
          const SizedBox(height: 12),
          Row(
            children: [
              const Icon(Icons.attach_file, size: 18, color: Colors.grey),
              const SizedBox(width: 4),
              Text(
                '简历.pdf${e.extra.isNotEmpty ? ' + ${e.extra}' : ''}',
                style: TextStyle(fontSize: 13, color: Colors.grey.shade700),
              ),
            ],
          ),
          const SizedBox(height: 16),
          _screeningCard(e, dc, dbg),
          const SizedBox(height: 16),
          _buildManualButtons(e),
          const SizedBox(height: 20),
          _buildActionPanel(e),
          if (_reportMarkdown != null) ...[
            const SizedBox(height: 20),
            _buildReportPreview(),
          ],
        ],
      ),
    );
  }

  Widget _bodyCard(Email e) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.grey.shade50,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        e.body.isEmpty ? '（无正文内容）' : e.body,
        style: TextStyle(
          fontSize: 14,
          height: 1.7,
          color: e.body.isEmpty ? Colors.grey.shade400 : null,
          fontStyle: e.body.isEmpty ? FontStyle.italic : null,
        ),
      ),
    );
  }

  Widget _screeningCard(Email e, Color dc, Color dbg) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: dbg,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: dc.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            e.detected == DetectResult.only ? '自动拒绝' : '自动通过',
            style: TextStyle(fontWeight: FontWeight.w700, color: dc),
          ),
          const SizedBox(height: 4),
          Text(
            e.detected == DetectResult.only
                ? '仅含简历附件，无正文/自荐内容。不符合筛选条件。'
                : '邮件包含正文或自荐信，进入人工流程。',
            style: TextStyle(fontSize: 13, color: dc),
          ),
        ],
      ),
    );
  }

  Widget _buildManualButtons(Email e) {
    return Row(
      children: [
        FilledButton.icon(
          onPressed:
              e.status == EmailStatus.passed || e.status == EmailStatus.rejected
              ? null
              : () => _mark(e.id, EmailStatus.passed),
          icon: const Icon(Icons.check, size: 18),
          label: const Text('通过'),
          style: FilledButton.styleFrom(
            backgroundColor: const Color(0xFF166534),
          ),
        ),
        const SizedBox(width: 8),
        OutlinedButton.icon(
          onPressed:
              e.status == EmailStatus.rejected || e.status == EmailStatus.passed
              ? null
              : () => _mark(e.id, EmailStatus.rejected),
          icon: const Icon(Icons.close, size: 18),
          label: const Text('拒绝'),
          style: OutlinedButton.styleFrom(
            foregroundColor: const Color(0xFF991B1B),
          ),
        ),
      ],
    );
  }

  Widget _buildActionPanel(Email e) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border.all(color: Colors.grey.shade300),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'qtrecurit access 动作',
            style: TextStyle(fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 8),
          Text(
            '每个按钮调用 provider Action API；dry_run 打开时只预览，不真实发送。',
            style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _interviewPositionController,
                  decoration: const InputDecoration(
                    labelText: '面试岗位',
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: TextField(
                  controller: _interviewTimeController,
                  decoration: const InputDecoration(
                    labelText: '面试时间',
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              _actionButton(
                e,
                RecruitmentAction.sendSurvey,
                Icons.assignment_outlined,
                '发送问卷',
              ),
              _actionButton(
                e,
                RecruitmentAction.sendTrainingInvite,
                Icons.group_add_outlined,
                '发送实训邀约',
              ),
              _actionButton(
                e,
                RecruitmentAction.sendExam,
                Icons.quiz_outlined,
                '发送笔试',
              ),
              _actionButton(
                e,
                RecruitmentAction.createInterviewNotice,
                Icons.event_available_outlined,
                '生成面试通知',
              ),
            ],
          ),
          const SizedBox(height: 12),
          _actionMapping('发送问卷', 'send_survey'),
          _actionMapping('发送实训邀约', 'send_training_invite'),
          _actionMapping('发送笔试', 'send_exam'),
          _actionMapping('生成面试通知', 'create_interview_notice'),
        ],
      ),
    );
  }

  Widget _actionButton(
    Email e,
    RecruitmentAction action,
    IconData icon,
    String label,
  ) {
    final running = _runningAction == action;
    return FilledButton.icon(
      onPressed: _runningAction != null
          ? null
          : () => _runCandidateAction(e, action),
      icon: running
          ? const SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(strokeWidth: 2),
            )
          : Icon(icon, size: 18),
      label: Text(running ? '执行中' : label),
    );
  }

  Widget _actionMapping(String label, String action) {
    return Padding(
      padding: const EdgeInsets.only(top: 2),
      child: Text(
        '后端动作：$action（$label）',
        style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
      ),
    );
  }

  Widget _buildReportPreview() {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFFF8FAFC),
        border: Border.all(color: Colors.grey.shade300),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '报告预览（$_reportStatus）',
            style: const TextStyle(fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 8),
          SelectableText(
            _reportMarkdown!,
            style: const TextStyle(
              fontFamily: 'monospace',
              fontSize: 13,
              height: 1.5,
            ),
          ),
        ],
      ),
    );
  }

  String _mockReportMarkdown() {
    return '# 招聘统计报告\n\n'
        '- 总邮件：$_total\n'
        '- 有正文：$_hasCount\n'
        '- 仅简历：$_onlyCount\n\n'
        '> mock fallback：未调用 provider。';
  }

  String _actionLabel(RecruitmentAction action) {
    switch (action) {
      case RecruitmentAction.sendSurvey:
        return '发送问卷';
      case RecruitmentAction.sendTrainingInvite:
        return '发送实训邀约';
      case RecruitmentAction.sendExam:
        return '发送笔试';
      case RecruitmentAction.createInterviewNotice:
        return '生成面试通知';
    }
  }

  String _actionDoneMessage(RecruitmentAction action) {
    switch (action) {
      case RecruitmentAction.sendSurvey:
        return '问卷邮件已发送';
      case RecruitmentAction.sendTrainingInvite:
        return '实训邀约已发送';
      case RecruitmentAction.sendExam:
        return '笔试邀请已发送';
      case RecruitmentAction.createInterviewNotice:
        return '面试通知草稿已生成';
    }
  }

  String _safeError(Object? error) {
    if (error is RecruitmentApiException) {
      return error.message;
    }
    return '招聘服务暂不可用，请稍后重试';
  }
}

String _positionFromSubject(String subject) {
  for (final position in [
    '前端开发',
    '后端开发',
    '产品经理',
    '数据分析师',
    'UI 设计师',
    '市场推广',
    '运营',
  ]) {
    if (subject.contains(position)) return position;
  }
  return '';
}

String _defaultInterviewTime() {
  final date = DateTime.now().add(const Duration(days: 3));
  final month = date.month.toString().padLeft(2, '0');
  final day = date.day.toString().padLeft(2, '0');
  return '${date.year}-$month-$day 10:00';
}
