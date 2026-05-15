import '../../models/entity.dart';

abstract class EntityDetector {
  List<Entity> detect(String text);
}
