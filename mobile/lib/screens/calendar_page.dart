import 'package:flutter/material.dart';

import '../widgets/k_app_bar.dart';
import 'calendar_screen.dart';

/// Family calendar destination (web: /calendar): activities + reminders layers.
class CalendarScreen extends StatelessWidget {
  const CalendarScreen({super.key});

  @override
  Widget build(BuildContext context) => Scaffold(appBar: kAppBar(context, 'Calendar'), body: const CalendarTab());
}
