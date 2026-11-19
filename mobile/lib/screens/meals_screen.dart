import 'package:flutter/material.dart';

import '../widgets/k_app_bar.dart';
import 'meal_plan_screen.dart';

/// Meals destination (web: /meals): this week's plan, must-use ingredients and
/// meal requests (backend-go/meal_plan.go).
class MealsScreen extends StatelessWidget {
  const MealsScreen({super.key});

  @override
  Widget build(BuildContext context) => Scaffold(appBar: kAppBar(context, 'Meals'), body: const MealsTab());
}
